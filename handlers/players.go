package handlers

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"encoding/json"
	"net/http"
)

func PlayersHandler(w http.ResponseWriter, r *http.Request) {

	league := r.URL.Query().Get("league")
	if league == "" {
		http.Error(w, "Informe a liga na URL:", http.StatusBadRequest)
		return
	}
	forceUpdate := r.URL.Query().Get("force_update")
	if forceUpdate != "true" {
		players, err := database.GetPlayersByLeague(league)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(players) > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(players)
			return
		}
	}
	apiPlayers, err := services.GetPlayers(league)
	if err != nil {
		http.Error(w, "Erro ao buscar API externa", http.StatusInternalServerError)
		return
	}
	for _, player := range apiPlayers {
		database.SavePlayer(player)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiPlayers)
}
