package handlers

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"net/http"
)

func SeasonsHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	if league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Liga necessária")
		return
	}

	seasons, err := database.GetAvailableSeasons(league)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar temporadas")
		return
	}
	utils.WriteJSON(w, http.StatusOK, seasons)
}
