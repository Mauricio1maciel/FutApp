package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// FetchESPNTeams lista os times de uma liga na ESPN (slug ex: "bra.1")
func FetchESPNTeams(espnLeagueSlug string) ([]models.ESPNTeam, error) {
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/teams", espnLeagueSlug)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ESPN respondeu %s", resp.Status)
	}

	var data struct {
		Sports []struct {
			Leagues []struct {
				Teams []struct {
					Team struct {
						ID               string `json:"id"`
						DisplayName      string `json:"displayName"`
						ShortDisplayName string `json:"shortDisplayName"`
						Abbreviation     string `json:"abbreviation"`
					} `json:"team"`
				} `json:"teams"`
			} `json:"leagues"`
		} `json:"sports"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("erro ao decodificar times da ESPN: %v", err)
	}

	var teams []models.ESPNTeam
	for _, sport := range data.Sports {
		for _, league := range sport.Leagues {
			for _, item := range league.Teams {
				id, err := strconv.ParseInt(item.Team.ID, 10, 64)
				if err != nil || id == 0 {
					continue
				}
				teams = append(teams, models.ESPNTeam{
					ID:           id,
					DisplayName:  item.Team.DisplayName,
					ShortName:    item.Team.ShortDisplayName,
					Abbreviation: item.Team.Abbreviation,
				})
			}
		}
	}
	return teams, nil
}

// SyncESPNTeamLinks percorre as ligas da football-data e vincula os times sem espn_team_id
func SyncESPNTeamLinks() (int, error) {
	leagues, err := database.GetLeaguesForTeamSync()
	if err != nil {
		return 0, err
	}

	total := 0
	for league, espnSlug := range leagues {
		espnTeams, err := FetchESPNTeams(espnSlug)
		if err != nil {
			utils.CustomLog("SYNC_TEAMS", "[%s] Erro ao buscar times na ESPN: %v", league, err)
			continue
		}

		linked, err := database.LinkESPNTeams(league, espnTeams)
		if err != nil {
			utils.CustomLog("SYNC_TEAMS", "[%s] Erro ao vincular times: %v", league, err)
			continue
		}
		total += linked

		time.Sleep(1 * time.Second) // Não martelar a ESPN
	}

	utils.CustomLog("SYNC_TEAMS", "✅ Sincronização concluída! %d times vinculados.", total)
	return total, nil
}
