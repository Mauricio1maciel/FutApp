package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// Rotinas que falam com as APIs externas e gravam no banco.
// Rodam no worker (Orange Pi) e, sob demanda, quando um admin pede update=true.

// FootballDataLeagues são as ligas cobertas pelo plano gratuito da football-data
var FootballDataLeagues = []string{"BSA", "PL", "PD", "SA", "BL1", "FL1", "CL", "CLI", "WC"}

// SyncFootballDataMatches baixa todos os jogos de uma liga na football-data e grava no banco
func SyncFootballDataMatches(ctx context.Context, league string) error {
	return SyncFootballDataSeason(ctx, league, 0)
}

// SyncFootballDataSeason é o SyncFootballDataMatches de uma temporada específica
// (seasonYear = ano em que ela começou; 0 = a atual). Serve para completar uma temporada
// que já acabou, como a Champions 2025-26, que ficou com o mata-mata sem resultado.
func SyncFootballDataSeason(ctx context.Context, league string, seasonYear int) error {
	apiMatches, err := GetMatchesByLeagueCode(league, seasonYear)
	if err != nil {
		return err
	}

	// Uma consulta só, em vez de uma por jogo
	format := database.GetLeagueSeasonFormat(ctx, league)

	rows := make([]database.MatchRow, 0, len(apiMatches))
	for _, m := range apiMatches {
		homeScore, awayScore := 0, 0
		var homePen, awayPen *int
		if m.Score.FullTime.Home != nil {
			homeScore = *m.Score.FullTime.Home
		}
		if m.Score.FullTime.Away != nil {
			awayScore = *m.Score.FullTime.Away
		}

		// O fullTime da football-data inclui os gols dos pênaltis
		if m.Score.Penalties.Home != nil && m.Score.Penalties.Away != nil {
			hP := *m.Score.Penalties.Home
			aP := *m.Score.Penalties.Away

			homePen = &hP
			awayPen = &aP

			homeScore = homeScore - hP
			awayScore = awayScore - aP
		}

		rows = append(rows, database.MatchRow{
			IDEvent: int64(m.ID), League: league, Season: SeasonFromDate(m.UTCDate, format),
			Round: m.Matchday, HomeTeamID: int64(m.HomeTeam.ID), AwayTeamID: int64(m.AwayTeam.ID),
			HomeScore: homeScore, AwayScore: awayScore, HomePenalty: homePen, AwayPenalty: awayPen,
			Date: m.UTCDate, Status: m.Status, Stage: m.Stage, Group: m.Group, Winner: m.Score.Winner,
		})
	}

	// A temporada inteira num comando só; os jogos iguais não são regravados
	changed, err := database.SaveMatches(ctx, rows)
	if ctx.Err() != nil {
		// Cancelado (ex: o pedido do admin caiu)
		utils.CustomLog("JOBS", "[%s] Football-Data interrompida: %v", league, ctx.Err())
		return ctx.Err()
	}
	if err != nil {
		// Algum jogo não foi gravado; os outros foram (gravação um por um). A classificação
		// segue sendo recalculada com o que está no banco
		utils.CustomLog("JOBS", "[%s] Football-Data: erro ao gravar jogos: %v", league, err)
	}
	utils.CustomLog("JOBS", "[%s] Football-Data: %d jogos, %d mudaram", league, len(rows), changed)
	return nil
}

// RecalculateStandings calcula a classificação a partir dos jogos do banco e salva.
// Não chama nenhuma API externa.
func RecalculateStandings(ctx context.Context, league, season string) error {
	matches, err := database.GetMatchesByLeague(ctx, league, season, database.MatchFilter{})
	if err != nil {
		return err
	}

	if league == "BSA" {
		// Campeões das copas da temporada: vagas na Libertadores sem cadastro manual
		syncBrazilianCupWinners(ctx, season)
	}
	// Liga de pontos corridos que vem da ESPN (ex: Série B): ela não informa a rodada,
	// então a rodada é calculada pelo calendário (services/rounds.go). Os jogos sem times
	// entram também: ficam na rodada 0
	if _, isCup := cupFormats[league]; !isCup && league != "UNL" && len(leagueTableMatches(league, matches)) > 0 {
		if espnOnly, _ := database.GetESPNOnlyLeagues(ctx); slices.Contains(espnOnly, league) {
			rounds := assignLeagueRounds(regularSeason(matches))
			if err := database.UpdateMatchRounds(ctx, rounds); err != nil {
				utils.CustomLog("JOBS", "[%s] Erro ao gravar as rodadas calculadas: %v", league, err)
			}
			// Os jogos fora das 38 rodadas (além do turno e returno, ou ainda sem times) são
			// os playoffs (Série B 2026: 3º a 6º): ganham a fase PLAYOFFS
			var playoffs []int64
			for id, r := range rounds {
				if r == 0 {
					playoffs = append(playoffs, id)
				}
			}
			if err := database.SetMatchesStage(ctx, playoffs, "PLAYOFFS"); err != nil {
				utils.CustomLog("JOBS", "[%s] Erro ao marcar os playoffs: %v", league, err)
			}
		}
	}

	// Regras lidas na hora (sem o cache da API): o worker grava a tabela logo depois de
	// atualizar os campeões das copas. Se alguma consulta falhar, não grava: uma tabela sem
	// as vagas sobrescreveria a certa até o próximo recálculo
	rules, err := loadTableRules(ctx, league, season)
	if err != nil {
		return fmt.Errorf("regras da tabela: %w", err)
	}
	standings, _, ok := buildTable(rules, league, season, matches, matches)
	if !ok {
		return nil // copa só de mata-mata (ex: Copa do Brasil): não tem tabela
	}
	return database.ReplaceStandings(ctx, league, season, standings)
}

// Copas com fase de grupos: classificação por grupo
var cupFormats = map[string]CupFormat{"WC": WorldCupFormat, "CLI": LibertadoresFormat, "CSU": SulAmericanaFormat}

// tableRules é o que a classificação de uma liga lê do banco, além dos jogos
type tableRules struct {
	winners  []models.Winner         // campeões das copas: vagas garantidas
	rule     *models.CompetitionRule // quantas vagas e quantos rebaixados
	zones    []models.CompetitionZone
	criteria []string // critérios de desempate
	espnOnly bool     // liga que vem só da ESPN (ex: Série B)
}

// loadTableRules lê as regras da tabela. As cinco consultas são independentes e vão ao
// banco ao mesmo tempo: do Render (EUA) ao banco (São Paulo), cada ida e volta custa
// ~185 ms, e em fila eram quase 1 s. Liga sem regra cadastrada (ex: Liga das Nações) não
// é erro: fica sem vagas.
func loadTableRules(ctx context.Context, league, season string) (tableRules, error) {
	var r tableRules
	var espnOnly []string
	errs := make([]error, 5)
	var wg sync.WaitGroup
	wg.Add(5)
	go func() {
		defer wg.Done()
		r.winners, errs[0] = database.GetWinnersBySeasonAndSeason(ctx, league, season)
	}()
	go func() {
		defer wg.Done()
		if r.rule, errs[1] = database.GetCompetitionRule(ctx, league, season); errors.Is(errs[1], sql.ErrNoRows) {
			errs[1] = nil
		}
	}()
	go func() {
		defer wg.Done()
		r.zones, errs[2] = database.GetZonesByLeague(ctx, league)
	}()
	go func() {
		defer wg.Done()
		r.criteria, errs[3] = database.GetTieBreakers(ctx, league, season)
	}()
	go func() {
		defer wg.Done()
		espnOnly, errs[4] = database.GetESPNOnlyLeagues(ctx)
	}()
	wg.Wait()
	r.espnOnly = slices.Contains(espnOnly, league)
	return r, errors.Join(errs...)
}

// As regras mudam poucas vezes por temporada (os campeões de copa, uma vez por final). A
// tabela ao vivo e as chances leem as regras em todo pedido que não está no cache da rota:
// guardadas por 2 minutos, viram uma ida ao banco de vez em quando.
var tableRulesCache = cache.New(2*time.Minute, 10*time.Minute)

// cachedTableRules é o loadTableRules das rotas de leitura. Erro não fica guardado.
func cachedTableRules(ctx context.Context, league, season string) (tableRules, error) {
	key := league + "|" + season
	if v, ok := tableRulesCache.Get(key); ok {
		return v.(tableRules), nil
	}
	r, err := loadTableRules(ctx, league, season)
	if err == nil {
		tableRulesCache.SetDefault(key, r)
	}
	return r, err
}

// buildTable monta a classificação da temporada, sem gravar. matches são os jogos que
// contam (na tabela ao vivo, os em andamento já entram como encerrados); formFrom são os
// jogos de verdade, de onde sai a forma recente. ok = false quando a competição não tem
// tabela (copa só de mata-mata). used são os jogos que entraram na tabela.
func buildTable(rules tableRules, league, season string, matches, formFrom []models.Match) (standings []models.Standing, used []models.Match, ok bool) {
	formMatches := formFrom // a forma usa os mesmos jogos que formam a tabela
	used = matches

	if format, isCup := cupFormats[league]; isCup {
		standings = BuildCupStandings(matches, rules.criteria, format)
		formMatches = groupStageMatches(formFrom)
		used = groupStageMatches(matches)
	} else if league == "UNL" {
		standings = BuildUNLStandings(matches, rules.criteria)
	} else {
		table := leagueTableMatches(league, matches)
		if len(table) == 0 && len(matches) > 0 {
			return nil, nil, false
		}
		formMatches = leagueTableMatches(league, formFrom)
		// Liga da ESPN (Série B): a ESPN dá aos playoffs a mesma fase dos pontos corridos.
		// Os jogos além do turno e returno não podem somar na tabela nem na forma
		if rules.espnOnly {
			table = withoutMatches(table, beyondRoundRobin(table))
			formMatches = withoutMatches(formMatches, beyondRoundRobin(formMatches))
		}
		standings = BuildStandings(table, rules.winners, rules.rule, rules.zones, rules.criteria)
		used = table
	}

	form := recentForm(formMatches)
	for i := range standings {
		standings[i].Season = season
		standings[i].Form = form[standings[i].TeamID]
	}
	return standings, used, true
}

// recentForm devolve os últimos 5 resultados de cada time ("VVEDV", do mais antigo para
// o mais recente). Pênaltis não contam: o jogo terminou empatado.
func recentForm(matches []models.Match) map[int64]string {
	finished := make([]models.Match, 0, len(matches))
	for _, m := range matches {
		if m.Status == "FINISHED" {
			finished = append(finished, m)
		}
	}
	// DateEvent vem do banco como "2026-10-11 21:30:00": a ordem do texto é a da data
	sort.SliceStable(finished, func(i, j int) bool { return finished[i].DateEvent < finished[j].DateEvent })

	form := map[int64]string{}
	add := func(team int64, r string) {
		f := form[team] + r
		if len(f) > 5 {
			f = f[len(f)-5:]
		}
		form[team] = f
	}
	for _, m := range finished {
		switch {
		case m.HomeScore > m.AwayScore:
			add(m.APIHomeTeamID, "V")
			add(m.APIAwayTeamID, "D")
		case m.HomeScore < m.AwayScore:
			add(m.APIHomeTeamID, "D")
			add(m.APIAwayTeamID, "V")
		default:
			add(m.APIHomeTeamID, "E")
			add(m.APIAwayTeamID, "E")
		}
	}
	return form
}

func withoutMatches(matches []models.Match, drop map[int64]bool) []models.Match {
	if len(drop) == 0 {
		return matches
	}
	out := make([]models.Match, 0, len(matches))
	for _, m := range matches {
		if !drop[m.MatchID] {
			out = append(out, m)
		}
	}
	return out
}

func regularSeason(matches []models.Match) []models.Match {
	out := make([]models.Match, 0, len(matches))
	for _, m := range matches {
		if m.Stage == "" || m.Stage == "REGULAR_SEASON" {
			out = append(out, m)
		}
	}
	return out
}

func groupStageMatches(matches []models.Match) []models.Match {
	group := make([]models.Match, 0, len(matches))
	for _, m := range matches {
		if m.Stage == "GROUP_STAGE" {
			group = append(group, m)
		}
	}
	return group
}

// leagueTableMatches separa os jogos que contam na tabela de pontos corridos
func leagueTableMatches(league string, matches []models.Match) []models.Match {
	valid := make([]models.Match, 0, len(matches))
	for _, m := range matches {
		// Na Champions, o mata-mata (PLAYOFFS, LAST_16...) não soma pontos na fase de
		// liga. Fase vazia: jogos gravados sem fase, todos de pontos corridos
		switch m.Stage {
		case "", "REGULAR_SEASON", "LEAGUE_STAGE":
		default:
			continue
		}
		// Jogos com time fora da tabela teams (ex: IDs da ESPN gravados por engano)
		// criariam times fantasmas e deslocariam a zona de rebaixamento
		if m.HomeTeam == "" || m.AwayTeam == "" {
			// ID 0 = confronto de mata-mata ainda indefinido, não é erro
			if m.APIHomeTeamID != 0 && m.APIAwayTeamID != 0 {
				utils.CustomLog("DATABASE_ERRO", "Jogo %s ignorado na classificação de %s: time desconhecido (%d x %d)", m.IDEvent, league, m.APIHomeTeamID, m.APIAwayTeamID)
			}
			continue
		}
		valid = append(valid, m)
	}
	return valid
}

// SyncFootballDataHistory carrega uma temporada antiga da football-data (times e jogos) e
// calcula a classificação dela. Serve ao confronto direto e ao histórico de temporadas.
// Os times entram sem sobrescrever os que já existem: nome e escudo atuais ficam.
func SyncFootballDataHistory(ctx context.Context, league string, seasonYear int) error {
	teams, err := GetTeams(league, seasonYear)
	if err != nil {
		return fmt.Errorf("times: %w", err)
	}
	format := database.GetLeagueSeasonFormat(ctx, league)
	season := SeasonFromDate(fmt.Sprintf("%d-08-01T00:00:00Z", seasonYear), format)
	for _, t := range teams {
		database.SaveHistoricalTeam(ctx, int64(t.ID), t.Name, t.Short, t.TLA, league, t.Stadium, t.Crest, season)
	}

	time.Sleep(7 * time.Second) // limite de 10 chamadas por minuto do plano grátis
	if err := SyncFootballDataSeason(ctx, league, seasonYear); err != nil {
		return fmt.Errorf("jogos: %w", err)
	}
	return RecalculateStandings(ctx, league, season)
}

// SyncFootballDataTeams grava os times e os elencos de uma liga da football-data
func SyncFootballDataTeams(ctx context.Context, league string) error {
	season := CurrentSeason(ctx, league)

	teams, err := GetTeams(league, 0)
	if err != nil {
		return fmt.Errorf("times: %w", err)
	}
	for _, t := range teams {
		database.SaveTeam(ctx, int64(t.ID), t.Name, t.Short, t.TLA, league, t.Stadium, t.Crest, season)
	}

	players, err := GetPlayers(league)
	if err != nil {
		return fmt.Errorf("elencos: %w", err)
	}
	for _, p := range players {
		database.SavePlayer(ctx, p)
	}

	utils.CustomLog("JOBS", "[%s] Football-Data: %d times e %d jogadores gravados", league, len(teams), len(players))
	return nil
}

// SyncAllESPNRosters atualiza o elenco (fotos, números, posições) de todos os times
// vinculados à ESPN
func SyncAllESPNRosters(ctx context.Context) {
	teams, err := database.GetESPNTeamsForRosterSync(ctx)
	if err != nil {
		utils.CustomLog("JOBS", "Erro ao listar times para elencos da ESPN: %v", err)
		return
	}

	ok := 0
	for _, t := range teams {
		if err := SyncESPNRoster(ctx, t.ESPNLeague, int(t.ESPNTeamID)); err != nil {
			utils.CustomLog("JOBS", "Elenco ESPN do time %d falhou: %v", t.ESPNTeamID, err)
		} else {
			ok++
		}
		time.Sleep(2 * time.Second) // Não martelar a ESPN
	}
	utils.CustomLog("JOBS", "Elencos da ESPN: %d/%d times atualizados", ok, len(teams))
}
