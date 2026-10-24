package handlers

import (
	"App-Futebol/services"
	"App-Futebol/utils"
	"net/http"
	"strconv"
	"strings"
)

// TeamCardsHandler: pendurados e suspensos do time na competição.
// GET /team/cards?id=1783&league=BSA[&season=2026]
func TeamCardsHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	teamID, err := strconv.ParseInt(q.Get("id"), 10, 64)
	league := strings.ToUpper(strings.TrimSpace(q.Get("league")))
	if err != nil || teamID <= 0 || league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe o time e a competição: ?id=1783&league=BSA")
		return
	}
	if !services.HasCardRule(league) {
		utils.WriteError(w, http.StatusNotFound, "A regra de cartões desta competição ainda não está cadastrada (por enquanto, Séries A e B)")
		return
	}
	season := services.ResolveSeason(r.Context(), league, q.Get("season"))

	result, err := services.TeamCardsFor(r.Context(), league, season, teamID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar os cartões do time")
		return
	}
	utils.WriteJSON(w, http.StatusOK, result)
}
