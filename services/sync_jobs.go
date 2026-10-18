package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"fmt"
	"slices"
	"sort"
	"time"
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

	saved := 0
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

		err := database.SaveMatch(ctx,
			int64(m.ID), league,
			SeasonFromDate(m.UTCDate, format),
			m.Matchday,
			int64(m.HomeTeam.ID), int64(m.AwayTeam.ID),
			homeScore, awayScore,
			homePen, awayPen,
			m.UTCDate, m.Status,
			m.Stage, m.Group,
			m.Score.Winner,
		)
		if err == nil {
			saved++
		}
	}

	utils.CustomLog("JOBS", "[%s] Football-Data: %d/%d jogos gravados", league, saved, len(apiMatches))
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
	winners, _ := database.GetWinnersBySeasonAndSeason(ctx, league, season)
	rule, _ := database.GetCompetitionRule(ctx, league, season)
	zones, _ := database.GetZonesByLeague(ctx, league)
	criteria, _ := database.GetTieBreakers(ctx, league, season)

	var standings []models.Standing

	// Jogos que formam a tabela: a forma recente usa os mesmos
	formMatches := matches

	if league == "WC" || league == "CLI" || league == "CSU" {
		format := map[string]CupFormat{"WC": WorldCupFormat, "CLI": LibertadoresFormat, "CSU": SulAmericanaFormat}[league]
		standings = BuildCupStandings(matches, criteria, format)
		formMatches = groupStageMatches(matches)
	} else if league == "UNL" {
		standings = BuildUNLStandings(matches, criteria)
	} else {
		table := leagueTableMatches(league, matches)
		if len(table) == 0 && len(matches) > 0 {
			// Copa só de mata-mata (ex: Copa do Brasil): não tem tabela
			return nil
		}
		// Liga de pontos corridos que vem da ESPN (ex: Série B): ela não informa a rodada,
		// então a rodada é calculada pelo calendário (services/rounds.go)
		if espnOnly, _ := database.GetESPNOnlyLeagues(ctx); slices.Contains(espnOnly, league) {
			// Com os jogos sem times (table não tem): eles ficam na rodada 0
			if err := database.UpdateMatchRounds(ctx, assignLeagueRounds(regularSeason(matches))); err != nil {
				utils.CustomLog("JOBS", "[%s] Erro ao gravar as rodadas calculadas: %v", league, err)
			}
		}
		standings = BuildStandings(table, winners, rule, zones, criteria)
		formMatches = table
	}

	form := recentForm(formMatches)

	for i := range standings {
		standings[i].Season = season
		standings[i].Form = form[standings[i].TeamID]
	}

	return database.ReplaceStandings(ctx, league, season, standings)
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
