package handlers

import (
	"App-Futebol/database"
	"encoding/json"
	"net/http"
	"strings"
)

func CalendarHandler(w http.ResponseWriter, r *http.Request) {
	leaguesParam := r.URL.Query().Get("leagues") // ex: "WC,BSA,PL"
	month := r.URL.Query().Get("month")          // ex: "06"
	year := r.URL.Query().Get("year")            // ex: "2026"

	if leaguesParam == "" || month == "" || year == "" {
		http.Error(w, `{"error": "Faltam parâmetros: leagues, month, year"}`, http.StatusBadRequest)
		return
	}

	leagues := strings.Split(leaguesParam, ",")
	counts, err := database.GetCalendarCounts(leagues, month, year)
	if err != nil {
		http.Error(w, `{"error": "Erro ao buscar calendário"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(counts)
}
