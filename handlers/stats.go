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
	season := r.URL.Query().Get("season")

	if league == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Liga não informada"})
		return
	}
	if season == "" {
		season = database.GetLatestSeason(league)
		if season == "" {
			season = getSeasonByLeague(league)
		}
	}

	scorers, _ := database.GetTopStats(league, season, "goals")
	assists, _ := database.GetTopStats(league, season, "assists")

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

	json.NewEncoder(w).Encode(response)

	go services.SyncLeagueStatsBackground(league, season)
}
