package handlers

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/services"
	"encoding/json"
	"net/http"
)

func LeagueStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	league := r.URL.Query().Get("league")
	season := "2026" // Use a função database.GetLatestSeason(league) se já a tiver

	if league == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Liga não informada"})
		return
	}

	// 1. Pega do Banco de Dados (Velocidade Relâmpago!)
	scorers, _ := database.GetTopStats(league, season, "goals")
	assists, _ := database.GetTopStats(league, season, "assists")

	// Previne nil slices
	if scorers == nil {
		scorers = []models.PlayerStat{}
	}
	if assists == nil {
		assists = []models.PlayerStat{}
	}

	response := models.LeagueStatsResponse{
		TopScorers: scorers,
		TopAssists: assists,
	}

	// 2. Devolve para o App IMEDIATAMENTE
	json.NewEncoder(w).Encode(response)

	// 3. Vai buscar dados atualizados nas costas do utilizador
	go services.SyncLeagueStatsBackground(league, season)
}
