package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/services"
	"App-Futebol/utils"
	"net/http"
)

// StandingsHandler só lê a classificação do banco. O worker recalcula depois de cada
// sincronização de jogos; o admin pode forçar com update=true.
func StandingsHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	season := r.URL.Query().Get("season")
	forceUpdate := r.URL.Query().Get("update") == "true" && middlewares.IsAdmin(r)

	if league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe a liga")
		return
	}

	season = services.ResolveSeason(r.Context(), league, season)

	result, err := database.GetStandingsByLeague(r.Context(), league, season)

	// Recalcula se o admin pediu ou se a tabela ainda não existe (usa só o banco)
	if forceUpdate || (err == nil && len(result) == 0) {
		utils.CustomLog("API", "Calculando classificação: %s %s (admin=%v)", league, season, forceUpdate)
		if err := services.RecalculateStandings(r.Context(), league, season); err != nil {
			utils.CustomLog("DATABASE_ERRO", "Falha ao calcular classificação de %s: %v", league, err)
		}
		result, err = database.GetStandingsByLeague(r.Context(), league, season)
	}

	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar classificação")
		return
	}
	utils.WriteJSON(w, http.StatusOK, result)
}
