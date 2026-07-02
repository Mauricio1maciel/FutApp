package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func SyncLeagueStatsBackground(leagueCode string, season string) {
	utils.CustomLog("STATS", "Iniciando atualização de estatísticas para a liga %s...", leagueCode)
	espnLeague := getESPNLeague(leagueCode)

	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/statistics", espnLeague)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var espnData models.ESPNStatisticsResponse
	if err := json.NewDecoder(resp.Body).Decode(&espnData); err != nil {
		return
	}

	for _, statCategory := range espnData.Stats {
		for _, leader := range statCategory.Leaders {

			// 1. Captura o ID do Jogador e o ID do Time na competição (França na Copa, Real Madrid na Champions)
			playerID, _ := strconv.ParseInt(leader.Athlete.ID, 10, 64)
			teamID, _ := strconv.ParseInt(leader.Athlete.Team.ID, 10, 64)

			// 2. Alimenta a tabela cadastral de jogadores
			_ = database.UpsertESPNPlayerGeneric(
				playerID,
				leader.Athlete.DisplayName,
				leader.Athlete.Headshot.Href,
				teamID,
			)

			// 3. Soma as métricas numéricas
			var goals, assists, matches int
			for _, s := range leader.Athlete.Statistics {
				if s.Name == "totalGoals" {
					goals = int(s.Value)
				} else if s.Name == "goalAssists" {
					assists = int(s.Value)
				} else if s.Name == "appearances" {
					matches = int(s.Value)
				}
			}

			// 4. 🔥 Envia o playerID e o teamID correto da competição
			database.UpsertPlayerStat(playerID, teamID, leagueCode, season, goals, assists, matches)
		}
	}
	utils.CustomLog("STATS", "Estatísticas sincronizadas de forma normalizada!")
}
