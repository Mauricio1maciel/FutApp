package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/services"
	"App-Futebol/utils"
	"log"
	"net/http"
)

func MatchHistoryHandler(w http.ResponseWriter, r *http.Request) {

	matchID := r.URL.Query().Get("id")
	league := r.URL.Query().Get("league")
	// Buscar na ESPN durante a requisição é exclusivo do admin; o worker cuida do resto
	isAdmin := middlewares.IsAdmin(r)
	forceUpdate := r.URL.Query().Get("force_update") == "true" && isAdmin

	if matchID == "" || league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe o id e a league na URL")
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
		utils.WriteJSON(w, http.StatusOK, historyDB)
		return
	}

	historyDB, err := database.GetFullMatchFromDB(matchID)
	hasLineups := err == nil && historyDB != nil && len(historyDB.Lineups) > 0

	if hasLineups {
		utils.CustomLog("DATABASE", "Cache encontrado! Devolvendo JSON em milissegundos para o jogo %s", matchID)
		utils.WriteJSON(w, http.StatusOK, historyDB)
		return
	}

	if !isAdmin {
		if historyDB != nil {
			utils.WriteJSON(w, http.StatusOK, historyDB)
			return
		}
		utils.WriteError(w, http.StatusNotFound, "Jogo não disponível")
		return
	}

	utils.CustomLog("ESPN", "Sem escalação no DB. Admin buscando dados frescos na ESPN para o jogo %s...", matchID)
	match, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

	if err != nil {
		log.Printf("[ERRO ESPN FALLBACK] %v", err)
		if historyDB != nil {
			utils.WriteJSON(w, http.StatusOK, historyDB)
			return
		}
		utils.WriteError(w, http.StatusNotFound, "Jogo não disponível")
		return
	}

	errSave := database.SaveFullMatchHistory(match, lineups, events)
	if errSave != nil {
		utils.CustomLog("DATABASE_ERRO", "Falha ao salvar: %v", errSave)
	}

	fullHistory, errFetch := database.GetFullMatchFromDB(matchID)

	if errFetch == nil && fullHistory != nil {
		utils.WriteJSON(w, http.StatusOK, fullHistory)
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
		utils.WriteJSON(w, http.StatusOK, response)
	}
}
