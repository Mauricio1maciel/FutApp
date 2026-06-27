package handlers

import (
	"App-Futebol/database"
	"encoding/json"
	"net/http"
	"strconv"
)

func DetailsHandler(w http.ResponseWriter, r *http.Request) {
	apiIDStr := r.URL.Query().Get("api_id")
	entityType := r.URL.Query().Get("type")

	if apiIDStr == "" || entityType == "" {
		http.Error(w, "Informe api_id e type (team ou player) na URL.", http.StatusBadRequest)
		return
	}
	apiID, err := strconv.ParseInt(apiIDStr, 10, 64)
	if err != nil {
		http.Error(w, "api_id inválido. Deve ser um número.", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if entityType == "team" || entityType == "teams" {
		team, err := database.GetTeamByApiID(apiID)
		if err != nil {
			http.Error(w, "Erro ao buscar time", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(team)
		return

	} else if entityType == "players" {
		player, err := database.GetPlayerByApiID(apiID)
		if err != nil {
			http.Error(w, "Erro ao buscar jogador", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(player)
		return

	} else {
		http.Error(w, "Tipo inválido. Use type=team ou type=player.", http.StatusBadRequest)
	}
}
