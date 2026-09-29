package services

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"encoding/json"
	"fmt"
	"net/http"
)

func SyncESPNScoreboardForLeague(leagueCode string) {
	// Usa a função getESPNLeague que já existe no espn_service.go!
	espnLeague := getESPNLeague(leagueCode)

	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/scoreboard", espnLeague)

	resp, err := http.Get(url)
	if err != nil {
		utils.CustomLog("WORKER_ESPN", "Erro buscar scoreboard da liga %s: %v", leagueCode, err)
		return
	}
	defer resp.Body.Close()

	var data struct {
		Events []struct {
			ID           string `json:"id"`
			Competitions []struct {
				Status struct {
					Type struct {
						State string `json:"state"`
					} `json:"type"`
				} `json:"status"`
			} `json:"competitions"`
		} `json:"events"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		utils.CustomLog("WORKER_ESPN", "Erro no JSON do scoreboard da liga %s: %v", leagueCode, err)
		return
	}

	for _, event := range data.Events {
		if len(event.Competitions) > 0 {
			matchID := event.ID
			status := event.Competitions[0].Status.Type.State

			if status == "pre" || status == "in" || status == "post" {
				utils.CustomLog("WORKER_ESPN", "Registo Proativo: Puxando resumo do jogo %s (%s)", matchID, leagueCode)

				// Continua a passar o código curto (ex: UNL) para gravar no banco
				m, lineups, matchEvents, err := FetchAndParseESPNMatch(matchID, leagueCode)
				if err == nil {
					database.SaveFullMatchHistory(m, lineups, matchEvents)
				}
			}
		}
	}
}
