package services

import (
	"App-Futebol/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type FDTeamsWithSquadResponse struct {
	Teams []struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Squad []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Position    string `json:"position"`
			DateOfBirth string `json:"dateOfBirth"`
			Nationality string `json:"nationality"`
		} `json:"squad"`
	} `json:"teams"`
}

func GetPlayers(league string) ([]models.Player, error) {

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

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data FDTeamsWithSquadResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	var players []models.Player

	for _, t := range data.Teams {
		for _, p := range t.Squad {
			player := models.Player{
				ApiID:       p.ID,
				Name:        p.Name,
				Position:    p.Position,
				DateOfBirth: p.DateOfBirth,
				Nationality: p.Nationality,
				TeamID:      t.ID,
				TeamName:    t.Name,
				League:      league,
			}
			players = append(players, player)
		}
	}

	return players, nil
}
