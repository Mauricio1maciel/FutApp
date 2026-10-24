package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/services"
	"App-Futebol/utils"
	"errors"
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

// LiveStandingsHandler: a classificação como estaria se os jogos em andamento acabassem
// agora. GET /standings?league=BSA&live=true
func LiveStandingsHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	if league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe a liga")
		return
	}
	season := services.ResolveSeason(r.Context(), league, r.URL.Query().Get("season"))

	result, err := services.LiveStandings(r.Context(), league, season)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao montar a classificação ao vivo")
		return
	}
	utils.WriteJSON(w, http.StatusOK, result)
}

// ChancesHandler: as chances de título, de cada zona e de cada posição final, simulando o
// que falta da temporada. GET /standings/chances?league=BSA[&season=2026]
func ChancesHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	if league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe a liga")
		return
	}
	season := services.ResolveSeason(r.Context(), league, r.URL.Query().Get("season"))

	result, err := services.LeagueChancesFor(r.Context(), league, season)
	if errors.Is(err, services.ErrNoLeagueTable) {
		utils.WriteError(w, http.StatusNotFound, "Esta competição não tem tabela de pontos corridos")
		return
	}
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao calcular as chances")
		return
	}
	utils.WriteJSON(w, http.StatusOK, result)
}

// ViewStandingsHandler: a classificação de um recorte da temporada.
// GET /standings?league=BSA&view=home|away|first|second
func ViewStandingsHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	if league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe a liga")
		return
	}
	season := services.ResolveSeason(r.Context(), league, r.URL.Query().Get("season"))

	result, err := services.ViewStandings(r.Context(), league, season, r.URL.Query().Get("view"))
	switch {
	case errors.Is(err, services.ErrUnknownView):
		utils.WriteError(w, http.StatusBadRequest, "view deve ser home (como mandante), away (como visitante), first (1º turno) ou second (2º turno)")
	case errors.Is(err, services.ErrNoLeagueTable):
		utils.WriteError(w, http.StatusNotFound, "Esta competição não tem tabela de pontos corridos")
	case err != nil:
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao montar a classificação")
	default:
		utils.WriteJSON(w, http.StatusOK, result)
	}
}
