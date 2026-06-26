// package handlers

// import (
// 	"App-Futebol/database"
// 	"App-Futebol/services"
// 	"App-Futebol/utils"
// 	"encoding/json"
// 	"log"
// 	"net/http"
// )

// func MatchHistoryHandler(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")

// 	matchID := r.URL.Query().Get("id")
// 	league := r.URL.Query().Get("league")
// 	forceUpdate := r.URL.Query().Get("force_update")
// 	nocache := r.URL.Query().Get("nocache")

// 	if matchID == "" || league == "" {
// 		w.WriteHeader(http.StatusBadRequest)
// 		json.NewEncoder(w).Encode(map[string]string{"error": "Informe o id e a league na URL"})
// 		return
// 	}

// 	if forceUpdate != "true" && nocache == "" {
// 		historyDB, err := database.GetFullMatchFromDB(matchID)
// 		if err == nil && historyDB != nil && len(historyDB.Lineups) > 0 {
// 			utils.CustomLog("DATABASE", "Cache encontrado para o jogo %s", matchID)
// 			json.NewEncoder(w).Encode(historyDB)
// 			return
// 		}
// 	}

// 	utils.CustomLog("ESPN", "Buscando dados frescos na ESPN para o jogo %s...", matchID)
// 	match, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

// 	if err != nil {
// 		log.Printf("[ERRO ESPN] %v", err)
// 		historyDB, _ := database.GetFullMatchFromDB(matchID)
// 		if historyDB != nil {
// 			json.NewEncoder(w).Encode(historyDB)
// 			return
// 		}
// 		w.WriteHeader(http.StatusNotFound)
// 		json.NewEncoder(w).Encode(map[string]string{"error": "Jogo não disponível"})
// 		return
// 	}

// 	errSave := database.SaveFullMatchHistory(match, lineups, events)
// 	if errSave != nil {
// 		utils.CustomLog("DATABASE_ERRO", "Falha ao salvar: %v", errSave)
// 	}

// 	fullHistory, errFetch := database.GetFullMatchFromDB(matchID)

//		if errFetch == nil && fullHistory != nil {
//			utils.CustomLog("API", "Respondendo com dados enriquecidos do banco para %s", matchID)
//			json.NewEncoder(w).Encode(fullHistory)
//		} else {
//			response := struct {
//				Match   interface{} `json:"match"`
//				Lineups interface{} `json:"lineups"`
//				Events  interface{} `json:"events"`
//			}{
//				Match:   match,
//				Lineups: lineups,
//				Events:  events,
//			}
//			json.NewEncoder(w).Encode(response)
//		}
//	}
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
	forceUpdate := r.URL.Query().Get("force_update")
	nocache := r.URL.Query().Get("nocache")

	if matchID == "" || league == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Informe o id e a league na URL"})
		return
	}

	if forceUpdate != "true" && nocache == "" {
		historyDB, err := database.GetFullMatchFromDB(matchID)
		if err == nil && historyDB != nil && len(historyDB.Lineups) > 0 {
			// 🔥 SE A PARTIDA ESTÁ NO CACHE MAS ESTÁ AO VIVO, VAMOS BUSCAR O RELÓGIO ATUALIZADO
			if historyDB.Match.Status == "in" {
				utils.CustomLog("ESPN", "Jogo no cache está Ao Vivo. Atualizando o clock para %s", matchID)
				services.UpdateLiveMatchClock(&historyDB.Match) // Função nova que criaremos!
			}
			utils.CustomLog("DATABASE", "Cache encontrado para o jogo %s", matchID)
			json.NewEncoder(w).Encode(historyDB)
			return
		}
	}

	utils.CustomLog("ESPN", "Buscando dados frescos na ESPN para o jogo %s...", matchID)
	match, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

	if err != nil {
		log.Printf("[ERRO ESPN] %v", err)
		historyDB, _ := database.GetFullMatchFromDB(matchID)
		if historyDB != nil {
			json.NewEncoder(w).Encode(historyDB)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Jogo não disponível"})
		return
	}

	// 🔥 BUSCA O RELÓGIO ANTES DE SALVAR SE ESTIVER AO VIVO
	if match.Status == "in" {
		services.UpdateLiveMatchClock(&match)
	}

	errSave := database.SaveFullMatchHistory(match, lineups, events)
	if errSave != nil {
		utils.CustomLog("DATABASE_ERRO", "Falha ao salvar: %v", errSave)
	}

	fullHistory, errFetch := database.GetFullMatchFromDB(matchID)

	if errFetch == nil && fullHistory != nil {
		// 🔥 GARANTE QUE O RELÓGIO ATUALIZADO VÁ PRO FRONTEND
		if fullHistory.Match.Status == "in" {
			fullHistory.Match.Clock = match.Clock
		}
		utils.CustomLog("API", "Respondendo com dados enriquecidos do banco para %s", matchID)
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
