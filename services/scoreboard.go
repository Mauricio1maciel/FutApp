package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"encoding/json"
	"fmt"
	"strconv"
)

// SyncESPNScoreboardForLeague registra os jogos do dia de uma liga na espn_matches,
// para o worker encontrá-los (escalações no pré-jogo, placar ao vivo).
//
// Para economizar chamadas à ESPN, o próprio scoreboard já traz times, placar,
// status, grupo e fase. O summary (escalação e eventos) só é buscado para jogos
// ao vivo ou encerrados que ainda não estão completos no banco.
func SyncESPNScoreboardForLeague(leagueCode string) {
	espnLeague := getESPNLeague(leagueCode)
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/scoreboard", espnLeague)

	resp, err := httpClient.Get(url)
	if err != nil {
		utils.CustomLog("WORKER_ESPN", "Erro buscar scoreboard da liga %s: %v", leagueCode, err)
		return
	}
	defer resp.Body.Close()

	var data models.ESPNScoreboard
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		utils.CustomLog("WORKER_ESPN", "Erro no JSON do scoreboard da liga %s: %v", leagueCode, err)
		return
	}

	season := ""
	if len(data.Leagues) > 0 {
		season = seasonFromESPN(data.Leagues[0].Season.Year, data.Leagues[0].Season.DisplayName)
	}

	var ids []int64
	for _, event := range data.Events {
		if id, err := strconv.ParseInt(event.ID, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	complete, err := database.GetCompleteESPNMatchIDs(ids)
	if err != nil {
		utils.CustomLog("WORKER_ESPN", "Erro ao consultar jogos completos (%s): %v", leagueCode, err)
		complete = map[string]bool{}
	}

	summaries := 0
	for _, event := range data.Events {
		if len(event.Competitions) == 0 {
			continue
		}
		basic := matchFromScoreboardEvent(event, leagueCode, season)

		needsSummary := basic.Status == "in" || (basic.Status == "post" && !complete[basic.MatchID])
		if !needsSummary {
			// Pré-jogo ou encerrado já completo: grava só o básico, sem chamar o summary
			database.SaveFullMatchHistory(basic, nil, nil)
			continue
		}

		m, lineups, matchEvents, err := FetchAndParseESPNMatch(basic.MatchID, leagueCode)
		summaries++
		if err != nil {
			utils.CustomLog("WORKER_ESPN", "Erro no summary do jogo %s: %v", basic.MatchID, err)
			database.SaveFullMatchHistory(basic, nil, nil)
			continue
		}

		// A fase só vem no scoreboard; o grupo também é mais confiável aqui
		if basic.Stage != "" {
			m.Stage = basic.Stage
		}
		if basic.GroupName != "" {
			m.GroupName = basic.GroupName
		}
		database.SaveFullMatchHistory(m, lineups, matchEvents)
	}

	utils.CustomLog("WORKER_ESPN", "[%s] Scoreboard: %d jogos, %d summaries buscados", leagueCode, len(data.Events), summaries)
}

// matchFromScoreboardEvent monta o jogo só com os dados do scoreboard (sem escalação/eventos)
func matchFromScoreboardEvent(event models.ESPNEvent, leagueCode, season string) models.ESPNMatchDB {
	comp := event.Competitions[0]
	m := models.ESPNMatchDB{
		MatchID:   event.ID,
		League:    leagueCode,
		Season:    season,
		MatchDate: event.Date,
		Status:    comp.Status.Type.State,
		Stage:     event.Season.Slug,
		GroupName: comp.Group.Name,
	}

	for _, c := range comp.Competitors {
		teamID, _ := strconv.ParseInt(c.Team.ID, 10, 64)
		if c.HomeAway == "home" {
			m.ESPNHomeTeamID, m.HomeTeam, m.HomeLogo, m.HomeScore = teamID, c.Team.DisplayName, c.Team.Logo, c.Score
		} else {
			m.ESPNAwayTeamID, m.AwayTeam, m.AwayLogo, m.AwayScore = teamID, c.Team.DisplayName, c.Team.Logo, c.Score
		}
	}
	return m
}
