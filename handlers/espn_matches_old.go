package handlers

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"App-Futebol/utils"
	"net/http"
	"strconv"
)

func SyncPastMatchHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	date := r.URL.Query().Get("date")
	homeIDStr := r.URL.Query().Get("espn_home_team_id")
	awayIDStr := r.URL.Query().Get("espn_away_team_id")

	if league == "" || date == "" || homeIDStr == "" || awayIDStr == "" {
		utils.WriteError(w, http.StatusBadRequest, "Parâmetros incompletos")
		return
	}

	homeID, _ := strconv.ParseInt(homeIDStr, 10, 64)
	awayID, _ := strconv.ParseInt(awayIDStr, 10, 64)

	espnMatchID, err := services.FindESPNMatchID(league, date, homeID, awayID)
	if err != nil || espnMatchID == "" {
		utils.WriteError(w, http.StatusNotFound, "Jogo não encontrado na ESPN para esta data")
		return
	}
	match, lineups, events, err := services.FetchAndParseESPNMatch(espnMatchID, league)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Falha ao baixar detalhes da ESPN")
		return
	}
	err = database.SaveFullMatchHistoryold(match, lineups, events)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Falha ao persistir dados no banco")
		return
	}
	fullData, err := database.GetFullMatchFromDB(espnMatchID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao resgatar dados salvos")
		return
	}
	utils.WriteJSON(w, http.StatusOK, fullData)
}
