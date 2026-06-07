package services

import (
	"App-Futebol/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type FDTeamsResponse struct {
	Teams []struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Short string `json:"shortName"`
		TLA   string `json:"tla"`
		Venue string `json:"venue"`
		Crest string `json:"crest"`
	} `json:"teams"`
}

func GetTeams(league string) ([]models.Team, error) {

	url := fmt.Sprintf(
		"https://api.football-data.org/v4/competitions/%s/teams",
		league,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	token := os.Getenv("API_TOKEN")
	req.Header.Set("X-Auth-Token", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data FDTeamsResponse

	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	var teams []models.Team

	for _, t := range data.Teams {

		team := models.Team{
			ID:      t.ID,
			Name:    t.Name,
			Short:   t.Short,
			TLA:     t.TLA,
			League:  league,
			Stadium: t.Venue,
			Crest:   t.Crest,
		}

		teams = append(teams, team)
	}

	return teams, nil
}
