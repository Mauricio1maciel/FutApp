package handlers

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
)

func DetailsHandler(w http.ResponseWriter, r *http.Request) {
	apiIDStr := r.URL.Query().Get("api_id")
	entityType := r.URL.Query().Get("type")

	if apiIDStr == "" || entityType == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe api_id e type (team ou player) na URL.")
		return
	}
	apiID, err := strconv.ParseInt(apiIDStr, 10, 64)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "api_id inválido. Deve ser um número.")
		return
	}
	if entityType == "team" || entityType == "teams" {
		team, err := database.GetTeamByApiID(r.Context(), apiID)
		if errors.Is(err, sql.ErrNoRows) {
			utils.WriteError(w, http.StatusNotFound, "Time não encontrado")
			return
		}
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar time")
			return
		}
		utils.WriteJSON(w, http.StatusOK, team)
		return

	} else if entityType == "player" || entityType == "players" {
		player, err := database.GetPlayerByApiID(r.Context(), apiID)
		if errors.Is(err, sql.ErrNoRows) {
			utils.WriteError(w, http.StatusNotFound, "Jogador não encontrado")
			return
		}
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar jogador")
			return
		}
		utils.WriteJSON(w, http.StatusOK, player)
		return

	} else {
		utils.WriteError(w, http.StatusBadRequest, "Tipo inválido. Use type=team ou type=player.")
	}
}
