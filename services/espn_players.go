package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
)

func SyncESPNRoster(ctx context.Context, leagueESPNSlug string, espnTeamID int) error {
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%v/teams/%v/roster", leagueESPNSlug, espnTeamID)

	resp, err := httpClient.Get(url)
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
			// Só vem para quem tem foto na ESPN (no Arsenal, 2 de 29 jogadores)
			Headshot struct {
				Href string `json:"href"`
			} `json:"headshot"`
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

		// Antes o link era montado pelo ID para todo jogador, e 9 em cada 10 davam 404.
		// Sem foto na ESPN fica vazio: o robô de imagens procura, e o app mostra as iniciais
		photoURL := item.Headshot.Href

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

		database.UpsertESPNPlayer(ctx, player)
	}

	return nil
}
