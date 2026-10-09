package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/services"
	"App-Futebol/utils"
	"net/http"
)

func TeamsHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	season := r.URL.Query().Get("season")
	// Times vêm do banco (o worker sincroniza diariamente); só o admin força a football-data
	forceUpdate := r.URL.Query().Get("update") == "true" && middlewares.IsAdmin(r)

	if league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe a league")
		return
	}

	if season == "" {
		season = services.CurrentSeason(r.Context(), league)
	}
	if !forceUpdate {
		teams, err := database.GetTeamsByLeague(r.Context(), league)
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar times")
			return
		}
		utils.WriteJSON(w, http.StatusOK, teams)
		return
	}

	teams, err := services.GetTeams(league)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, "Erro ao buscar times na football-data")
		return
	}

	savedCount := 0
	for _, team := range teams {
		err := database.SaveTeam(r.Context(),
			int64(team.ID),
			team.Name,
			team.Short,
			team.TLA,
			league,
			team.Stadium,
			team.Crest,
			season)
		if err == nil {
			savedCount++
		}
	}
	utils.CustomLog("API", "Admin atualizou %d times da liga %s", savedCount, league)

	updated, err := database.GetTeamsByLeague(r.Context(), league)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar times")
		return
	}
	utils.WriteJSON(w, http.StatusOK, updated)
}
