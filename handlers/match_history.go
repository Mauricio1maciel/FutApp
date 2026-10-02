package handlers

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"App-Futebol/utils"
	"encoding/json"
	"log"
	"net/http"
)

func MatchHistoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	matchID := r.URL.Query().Get("id")
	league := r.URL.Query().Get("league")
	forceUpdate := r.URL.Query().Get("force_update") == "true"

	if matchID == "" || league == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Informe o id e a league na URL"})
		return
	}

	if forceUpdate {
		utils.CustomLog("ESPN", "Pull to Refresh! Forçando atualização do jogo %s na ESPN...", matchID)
		match, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

		if err == nil {
			database.SaveFullMatchHistory(match, lineups, events)
		} else {
			log.Printf("[ERRO ESPN PULL TO REFRESH] %v", err)
		}

		historyDB, _ := database.GetFullMatchFromDB(matchID)
		json.NewEncoder(w).Encode(historyDB)
		return
	}

	historyDB, err := database.GetFullMatchFromDB(matchID)
	hasLineups := err == nil && historyDB != nil && len(historyDB.Lineups) > 0

	if hasLineups {
		utils.CustomLog("DATABASE", "Cache encontrado! Devolvendo JSON em milissegundos para o jogo %s", matchID)
		json.NewEncoder(w).Encode(historyDB)
		return
	}

	utils.CustomLog("ESPN", "Sem escalação no DB. Buscando dados frescos na ESPN para o jogo %s...", matchID)
	match, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

	if err != nil {
		log.Printf("[ERRO ESPN FALLBACK] %v", err)
		if historyDB != nil {
			json.NewEncoder(w).Encode(historyDB)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Jogo não disponível"})
		return
	}

	errSave := database.SaveFullMatchHistory(match, lineups, events)
	if errSave != nil {
		utils.CustomLog("DATABASE_ERRO", "Falha ao salvar: %v", errSave)
	}

	fullHistory, errFetch := database.GetFullMatchFromDB(matchID)

	if errFetch == nil && fullHistory != nil {
		json.NewEncoder(w).Encode(fullHistory)
	} else {
		response := struct {
			Match   interface{} `json:"match"`
			Lineups interface{} `json:"lineups"`
			Events  interface{} `json:"events"`
		}{
			Match:   match,
			Lineups: lineups,
			Events:  events,
		}
		json.NewEncoder(w).Encode(response)
	}
}
