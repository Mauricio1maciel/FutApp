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
// 			if historyDB.Match.Status == "in" {
// 				utils.CustomLog("ESPN", "Jogo no cache está Ao Vivo. Atualizando o clock para %s", matchID)
// 				services.UpdateLiveMatchClock(&historyDB.Match)
// 			}
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

// 	if match.Status == "in" {
// 		services.UpdateLiveMatchClock(&match)
// 	}

// 	errSave := database.SaveFullMatchHistory(match, lineups, events)
// 	if errSave != nil {
// 		utils.CustomLog("DATABASE_ERRO", "Falha ao salvar: %v", errSave)
// 	}

// 	fullHistory, errFetch := database.GetFullMatchFromDB(matchID)

//		if errFetch == nil && fullHistory != nil {
//			if fullHistory.Match.Status == "in" {
//				fullHistory.Match.Clock = match.Clock
//			}
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
	forceUpdate := r.URL.Query().Get("force_update") == "true"

	if matchID == "" || league == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Informe o id e a league na URL"})
		return
	}

	// 1. TENTA SEMPRE LER DO BANCO DE DADOS PRIMEIRO! (Isto é super rápido: ~50ms)
	historyDB, err := database.GetFullMatchFromDB(matchID)
	hasLineups := err == nil && historyDB != nil && len(historyDB.Lineups) > 0

	// 2. SE JÁ TEMOS A ESCALAÇÃO: Devolvemos IMEDIATAMENTE para a tela não travar!
	if hasLineups {
		if historyDB.Match.Status == "in" {
			// Atualiza só o relógio rápido antes de mandar, se o jogo estiver ao vivo
			utils.CustomLog("ESPN", "Jogo no cache está Ao Vivo. Atualizando o clock para %s", matchID)
			services.UpdateLiveMatchClock(&historyDB.Match)
		}

		utils.CustomLog("DATABASE", "Cache encontrado! Devolvendo JSON na hora para o jogo %s", matchID)
		json.NewEncoder(w).Encode(historyDB)

		// 🔥 A MÁGICA ACONTECE AQUI:
		// Se a tela pediu force_update ou o jogo está ao vivo, mandamos o servidor
		// buscar os novos eventos (gols, cartões) nas costas do utilizador!
		if forceUpdate || historyDB.Match.Status == "in" {
			go syncMatchDataBackground(matchID, league)
		}
		return
	}

	// 3. SE NÃO TEMOS A ESCALAÇÃO (Apenas na 1ª vez que o jogo é aberto):
	// O utilizador tem que esperar a busca na ESPN.
	utils.CustomLog("ESPN", "Sem escalação no DB. Buscando dados frescos na ESPN para o jogo %s...", matchID)
	match, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

	if err != nil {
		log.Printf("[ERRO ESPN] %v", err)
		// Se der erro na ESPN, mas temos os times no banco, devolve o que tem
		if historyDB != nil {
			json.NewEncoder(w).Encode(historyDB)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Jogo não disponível"})
		return
	}

	if match.Status == "in" {
		services.UpdateLiveMatchClock(&match)
	}

	// Salva no banco de dados
	errSave := database.SaveFullMatchHistory(match, lineups, events)
	if errSave != nil {
		utils.CustomLog("DATABASE_ERRO", "Falha ao salvar: %v", errSave)
	}

	// Pega do banco para já ir com os dados enriquecidos (logos locais, etc)
	fullHistory, errFetch := database.GetFullMatchFromDB(matchID)

	if errFetch == nil && fullHistory != nil {
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

// 🔥 FUNÇÃO DE SEGUNDO PLANO (Goroutine)
// Esta função roda sem travar a resposta HTTP. O App já recebeu a resposta
// enquanto o servidor faz o trabalho pesado de atualizar o banco de dados.
func syncMatchDataBackground(matchID string, league string) {
	utils.CustomLog("BACKGROUND", "Iniciando atualização oculta para o jogo %s", matchID)
	match, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

	if err == nil {
		database.SaveFullMatchHistory(match, lineups, events)
		utils.CustomLog("BACKGROUND", "Atualização oculta concluída com sucesso para o jogo %s", matchID)
	} else {
		log.Printf("[ERRO BACKGROUND] Falha ao atualizar ESPN para o jogo %s: %v", matchID, err)
	}
}
