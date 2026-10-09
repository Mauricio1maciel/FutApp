package handlers

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"net/http"
)

func GlobalSearchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	if query == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe o termo de busca (ex: ?q=nome)")
		return
	}

	leagues, err := database.SearchLeagues(r.Context(), query)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar ligas")
		return
	}

	teams, err := database.SearchTeamsGlobal(r.Context(), query)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar times")
		return
	}

	players, err := database.SearchPlayersGlobal(r.Context(), query)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar jogadores")
		return
	}

	result := models.SearchResult{
		Leagues: leagues,
		Teams:   teams,
		Players: players,
	}
	utils.WriteJSON(w, http.StatusOK, result)
}
