package handlers

import (
	"App-Futebol/database"
	"encoding/json"
	"net/http"
)

func SeasonsHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	if league == "" {
		http.Error(w, "Liga necessária", http.StatusBadRequest)
		return
	}

	seasons, err := database.GetAvailableSeasons(league)
	if err != nil {
		http.Error(w, "Erro ao buscar temporadas", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(seasons)
}
