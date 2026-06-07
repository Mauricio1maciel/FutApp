package handlers

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"encoding/json"
	"net/http"
)

func GlobalSearchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	if query == "" {
		http.Error(w, "Informe o termo de busca (ex: ?q=nome)", http.StatusBadRequest)
		return
	}

	teams, err := database.SearchTeamsGlobal(query)
	if err != nil {
		http.Error(w, "Erro ao buscar times", http.StatusInternalServerError)
		return
	}

	players, err := database.SearchPlayersGlobal(query)
	if err != nil {
		http.Error(w, "Erro ao buscar jogadores", http.StatusInternalServerError)
		return
	}

	result := models.SearchResult{
		Teams:   teams,
		Players: players,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
