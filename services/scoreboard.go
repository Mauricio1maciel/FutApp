package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func SyncESPNScoreboardForLeague(leagueCode string) {
	espnLeague := getESPNLeague(leagueCode) // Use a sua função que converte 'BSA' para 'bra.1'
	if espnLeague == "" {
		return
	}

	// Puxa o Scoreboard para a data de hoje (YYYYMMDD) para ser preciso
	today := time.Now().Format("20060102")
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/scoreboard?dates=%s", espnLeague, today)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var scoreboard models.ESPNScoreboardResponse
	if err := json.NewDecoder(resp.Body).Decode(&scoreboard); err != nil {
		return
	}

	for _, event := range scoreboard.Events {
		if len(event.Competitions) > 0 {
			var homeID, awayID string
			for _, comp := range event.Competitions[0].Competitors {
				if comp.HomeAway == "home" {
					homeID = comp.Team.ID
				} else if comp.HomeAway == "away" {
					awayID = comp.Team.ID
				}
			}

			// 🔥 Salva a ponte de ligação!
			if homeID != "" && awayID != "" {
				database.SaveESPNMatchMapping(event.ID, homeID, awayID, event.Date)
			}
		}
	}
	utils.CustomLog("SCOREBOARD", "Mapeamento atualizado para a liga %s", leagueCode)
}
