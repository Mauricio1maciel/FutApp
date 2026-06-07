package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

func SyncESPNRoster(leagueESPNSlug string, espnTeamID int) error {
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%v/teams/%v/roster", leagueESPNSlug, espnTeamID)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	var espnData struct {
		Athletes []struct {
			ID          string `json:"id"`
			FullName    string `json:"fullName"`
			ShortName   string `json:"shortName"`
			Jersey      string `json:"jersey"`
			DateOfBirth string `json:"dateOfBirth"`
			Position    struct {
				Name string `json:"name"`
			} `json:"position"`
			Flag struct {
				Alt string `json:"alt"`
			} `json:"flag"`
		} `json:"athletes"`
	}

	err = json.Unmarshal(body, &espnData)
	if err != nil {
		return fmt.Errorf("erro ao decodificar JSON da ESPN: %v", err)
	}

	for _, item := range espnData.Athletes {
		jerseyNum := 0
		fmt.Sscanf(item.Jersey, "%d", &jerseyNum)

		playerID, _ := strconv.Atoi(item.ID)

		photoURL := fmt.Sprintf("https://a.espncdn.com/combiner/i?img=/i/headshots/soccer/players/full/%v.png", item.ID)

		player := models.Player{
			ID:           playerID,
			Name:         item.FullName,
			ShortName:    item.ShortName,
			JerseyNumber: jerseyNum,
			DateOfBirth:  item.DateOfBirth,
			HeadshotURL:  photoURL,
			Position:     item.Position.Name,
			Nationality:  item.Flag.Alt,
			TeamID:       espnTeamID,
		}

		database.UpsertESPNPlayer(player)
	}

	return nil
}
