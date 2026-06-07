package handlers

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"encoding/json"
	"net/http"
	"strconv"
)

func SyncPastMatchHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	date := r.URL.Query().Get("date")
	homeIDStr := r.URL.Query().Get("espn_home_team_id")
	awayIDStr := r.URL.Query().Get("espn_away_team_id")

	if league == "" || date == "" || homeIDStr == "" || awayIDStr == "" {
		http.Error(w, `{"error": "Parâmetros incompletos"}`, http.StatusBadRequest)
		return
	}

	homeID, _ := strconv.ParseInt(homeIDStr, 10, 64)
	awayID, _ := strconv.ParseInt(awayIDStr, 10, 64)

	espnMatchID, err := services.FindESPNMatchID(league, date, homeID, awayID)
	if err != nil || espnMatchID == "" {
		http.Error(w, `{"error": "Jogo não encontrado na ESPN para esta data"}`, http.StatusNotFound)
		return
	}
	match, lineups, events, err := services.FetchAndParseESPNMatch(espnMatchID, league)
	if err != nil {
		http.Error(w, `{"error": "Falha ao baixar detalhes da ESPN"}`, http.StatusInternalServerError)
		return
	}
	err = database.SaveFullMatchHistoryold(match, lineups, events)
	if err != nil {
		http.Error(w, `{"error": "Falha ao persistir dados no banco"}`, http.StatusInternalServerError)
		return
	}
	fullData, err := database.GetFullMatchFromDB(espnMatchID)
	if err != nil {
		http.Error(w, `{"error": "Erro ao resgatar dados salvos"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fullData)
}
