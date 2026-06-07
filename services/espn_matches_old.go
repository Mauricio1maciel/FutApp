package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ESPNScoreboardResponse struct {
	Events []struct {
		ID           string `json:"id"`
		Competitions []struct {
			Competitors []struct {
				Team struct {
					ID string `json:"id"`
				} `json:"team"`
			} `json:"competitors"`
		} `json:"competitions"`
	} `json:"events"`
}

func FindESPNMatchID(leagueCode string, matchDate string, espnHomeTeamID int64, espnAwayTeamID int64) (string, error) {
	dateOnly := strings.Split(matchDate, " ")[0]
	dateFormatted := strings.ReplaceAll(dateOnly, "-", "")

	espnLeague := getESPNLeague(leagueCode)

	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/scoreboard?dates=%s", espnLeague, dateFormatted)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var data ESPNScoreboardResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	homeStr := fmt.Sprintf("%d", espnHomeTeamID)
	awayStr := fmt.Sprintf("%d", espnAwayTeamID)

	for _, event := range data.Events {
		if len(event.Competitions) > 0 {
			comp := event.Competitions[0]

			hasHome := false
			hasAway := false

			for _, competitor := range comp.Competitors {
				if competitor.Team.ID == homeStr {
					hasHome = true
				}
				if competitor.Team.ID == awayStr {
					hasAway = true
				}
			}

			if hasHome && hasAway {
				return event.ID, nil
			}
		}
	}

	return "", fmt.Errorf("jogo não encontrado na ESPN para a data %s", dateFormatted)
}
