package handlers

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"net/http"
	"strings"
)

func CalendarHandler(w http.ResponseWriter, r *http.Request) {
	leaguesParam := r.URL.Query().Get("leagues")
	month := r.URL.Query().Get("month")
	year := r.URL.Query().Get("year")

	if leaguesParam == "" || month == "" || year == "" {
		utils.WriteError(w, http.StatusBadRequest, "Faltam parâmetros: leagues, month, year")
		return
	}

	leagues := strings.Split(leaguesParam, ",")
	counts, err := database.GetCalendarCounts(leagues, month, year)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar calendário")
		return
	}
	utils.WriteJSON(w, http.StatusOK, counts)
}
