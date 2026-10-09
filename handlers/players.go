package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/services"
	"App-Futebol/utils"
	"net/http"
)

func PlayersHandler(w http.ResponseWriter, r *http.Request) {

	league := r.URL.Query().Get("league")
	if league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe a liga na URL:")
		return
	}
	// Elencos vêm do banco (o worker sincroniza diariamente); só o admin força a football-data
	forceUpdate := r.URL.Query().Get("force_update") == "true" && middlewares.IsAdmin(r)
	if !forceUpdate {
		players, err := database.GetPlayersByLeague(r.Context(), league)
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar jogadores")
			return
		}
		utils.WriteJSON(w, http.StatusOK, players)
		return
	}
	apiPlayers, err := services.GetPlayers(league)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, "Erro ao buscar jogadores na football-data")
		return
	}
	for _, player := range apiPlayers {
		database.SavePlayer(r.Context(), player)
	}

	// Devolve do banco para ter o mesmo formato da leitura normal
	players, err := database.GetPlayersByLeague(r.Context(), league)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar jogadores")
		return
	}
	utils.WriteJSON(w, http.StatusOK, players)
}
