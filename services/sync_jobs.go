package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"fmt"
	"time"
)

// Rotinas que falam com as APIs externas e gravam no banco.
// Rodam no worker (Orange Pi) e, sob demanda, quando um admin pede update=true.

// FootballDataLeagues são as ligas cobertas pelo plano gratuito da football-data
var FootballDataLeagues = []string{"BSA", "PL", "PD", "SA", "BL1", "FL1", "CL", "CLI", "WC"}

// SyncFootballDataMatches baixa todos os jogos de uma liga na football-data e grava no banco
func SyncFootballDataMatches(league string) error {
	apiMatches, err := GetMatchesByLeagueCode(league)
	if err != nil {
		return err
	}

	// Uma consulta só, em vez de uma por jogo
	format := database.GetLeagueSeasonFormat(league)

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

		err := database.SaveMatch(
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
func RecalculateStandings(league, season string) error {
	matches, err := database.GetMatchesByLeague(league, "", "", season, false)
	if err != nil {
		return err
	}

	winners, _ := database.GetWinnersBySeasonAndSeason(league, season)
	rule, _ := database.GetCompetitionRule(league, season)
	zones, _ := database.GetZonesByLeague(league)
	criteria, _ := database.GetTieBreakers(league, season)

	var standings []models.Standing

	if league == "WC" {
		standings = BuildCupStandings(matches, criteria, WorldCupFormat)
	} else if league == "CLI" {
		standings = BuildCupStandings(matches, criteria, LibertadoresFormat)
	} else if league == "UNL" {
		standings = BuildUNLStandings(matches, criteria)
	} else {
		// Jogos com time fora da tabela teams (ex: IDs da ESPN gravados por engano)
		// criariam times fantasmas e deslocariam a zona de rebaixamento
		validMatches := make([]models.Match, 0, len(matches))
		for _, m := range matches {
			if m.HomeTeam == "" || m.AwayTeam == "" {
				// ID 0 = confronto de mata-mata ainda indefinido, não é erro
				if m.APIHomeTeamID != 0 && m.APIAwayTeamID != 0 {
					utils.CustomLog("DATABASE_ERRO", "Jogo %s ignorado na classificação de %s: time desconhecido (%d x %d)", m.IDEvent, league, m.APIHomeTeamID, m.APIAwayTeamID)
				}
				continue
			}
			validMatches = append(validMatches, m)
		}
		standings = BuildStandings(validMatches, winners, rule, zones, criteria)
	}

	for i := range standings {
		standings[i].Season = season
	}

	return database.ReplaceStandings(league, season, standings)
}

// SyncFootballDataTeams grava os times e os elencos de uma liga da football-data
func SyncFootballDataTeams(league string) error {
	season := CurrentSeason(league)

	teams, err := GetTeams(league)
	if err != nil {
		return fmt.Errorf("times: %w", err)
	}
	for _, t := range teams {
		database.SaveTeam(int64(t.ID), t.Name, t.Short, t.TLA, league, t.Stadium, t.Crest, season)
	}

	players, err := GetPlayers(league)
	if err != nil {
		return fmt.Errorf("elencos: %w", err)
	}
	for _, p := range players {
		database.SavePlayer(p)
	}

	utils.CustomLog("JOBS", "[%s] Football-Data: %d times e %d jogadores gravados", league, len(teams), len(players))
	return nil
}

// SyncAllESPNRosters atualiza o elenco (fotos, números, posições) de todos os times
// vinculados à ESPN
func SyncAllESPNRosters() {
	teams, err := database.GetESPNTeamsForRosterSync()
	if err != nil {
		utils.CustomLog("JOBS", "Erro ao listar times para elencos da ESPN: %v", err)
		return
	}

	ok := 0
	for _, t := range teams {
		if err := SyncESPNRoster(t.ESPNLeague, int(t.ESPNTeamID)); err != nil {
			utils.CustomLog("JOBS", "Elenco ESPN do time %d falhou: %v", t.ESPNTeamID, err)
		} else {
			ok++
		}
		time.Sleep(2 * time.Second) // Não martelar a ESPN
	}
	utils.CustomLog("JOBS", "Elencos da ESPN: %d/%d times atualizados", ok, len(teams))
}
