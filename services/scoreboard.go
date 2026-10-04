package services

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"encoding/json"
	"fmt"
)

func SyncESPNScoreboardForLeague(leagueCode string) {
	espnLeague := getESPNLeague(leagueCode)
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/scoreboard", espnLeague)

	resp, err := httpClient.Get(url)
	if err != nil {
		utils.CustomLog("WORKER_ESPN", "Erro buscar scoreboard da liga %s: %v", leagueCode, err)
		return
	}
	defer resp.Body.Close()

	var data struct {
		Events []struct {
			ID     string `json:"id"`
			Season struct {
				Slug string `json:"slug"`
			} `json:"season"`
			Competitions []struct {
				Group struct {
					Name string `json:"name"`
				} `json:"group"`
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

				m, lineups, matchEvents, err := FetchAndParseESPNMatch(matchID, leagueCode)

				if err == nil {
					if event.Season.Slug != "" {
						m.Stage = event.Season.Slug
					}
					if event.Competitions[0].Group.Name != "" {
						m.GroupName = event.Competitions[0].Group.Name
					}

					// Agora sim, vai para o banco de dados com a informação completa!
					database.SaveFullMatchHistory(m, lineups, matchEvents)
				}
			}
		}
	}
}
