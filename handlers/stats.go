package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/models"
	"App-Futebol/services"
	"App-Futebol/utils"
	"net/http"
)

// LeagueStatsHandler só lê do banco. O worker sincroniza as estatísticas da ESPN
// a cada 6h; o admin pode forçar com update=true.
func LeagueStatsHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	season := r.URL.Query().Get("season")
	forceUpdate := r.URL.Query().Get("update") == "true" && middlewares.IsAdmin(r)

	if league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Liga não informada")
		return
	}
	season = services.ResolveSeason(league, season)

	if forceUpdate {
		services.SyncLeagueStatsBackground(league, season)
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

	utils.WriteJSON(w, http.StatusOK, response)
}
