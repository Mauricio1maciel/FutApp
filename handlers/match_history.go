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
// 	forceUpdate := r.URL.Query().Get("force_update") == "true"

// 	if matchID == "" || league == "" {
// 		w.WriteHeader(http.StatusBadRequest)
// 		json.NewEncoder(w).Encode(map[string]string{"error": "Informe o id e a league na URL"})
// 		return
// 	}

// 	// 1. TENTA SEMPRE LER DO BANCO DE DADOS PRIMEIRO! (Isto é super rápido: ~50ms)
// 	historyDB, err := database.GetFullMatchFromDB(matchID)
// 	hasLineups := err == nil && historyDB != nil && len(historyDB.Lineups) > 0

// 	// 2. SE JÁ TEMOS A ESCALAÇÃO: Devolvemos IMEDIATAMENTE para a tela não travar!
// 	if hasLineups {
// 		if historyDB.Match.Status == "in" {
// 			// Atualiza só o relógio rápido antes de mandar, se o jogo estiver ao vivo
// 			utils.CustomLog("ESPN", "Jogo no cache está Ao Vivo. Atualizando o clock para %s", matchID)
// 			services.UpdateLiveMatchClock(&historyDB.Match)
// 		}

// 		utils.CustomLog("DATABASE", "Cache encontrado! Devolvendo JSON na hora para o jogo %s", matchID)
// 		json.NewEncoder(w).Encode(historyDB)

// 		// 🔥 A MÁGICA ACONTECE AQUI:
// 		// Se a tela pediu force_update ou o jogo está ao vivo, mandamos o servidor
// 		// buscar os novos eventos (gols, cartões) nas costas do utilizador!
// 		if forceUpdate || historyDB.Match.Status == "in" {
// 			go syncMatchDataBackground(matchID, league)
// 		}
// 		return
// 	}

// 	// 3. SE NÃO TEMOS A ESCALAÇÃO (Apenas na 1ª vez que o jogo é aberto):
// 	// O utilizador tem que esperar a busca na ESPN.
// 	utils.CustomLog("ESPN", "Sem escalação no DB. Buscando dados frescos na ESPN para o jogo %s...", matchID)
// 	match, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

// 	if err != nil {
// 		log.Printf("[ERRO ESPN] %v", err)
// 		// Se der erro na ESPN, mas temos os times no banco, devolve o que tem
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

// 	// Salva no banco de dados
// 	errSave := database.SaveFullMatchHistory(match, lineups, events)
// 	if errSave != nil {
// 		utils.CustomLog("DATABASE_ERRO", "Falha ao salvar: %v", errSave)
// 	}

// 	// Pega do banco para já ir com os dados enriquecidos (logos locais, etc)
// 	fullHistory, errFetch := database.GetFullMatchFromDB(matchID)

// 	if errFetch == nil && fullHistory != nil {
// 		if fullHistory.Match.Status == "in" {
// 			fullHistory.Match.Clock = match.Clock
// 		}
// 		utils.CustomLog("API", "Respondendo com dados enriquecidos do banco para %s", matchID)
// 		json.NewEncoder(w).Encode(fullHistory)
// 	} else {
// 		response := struct {
// 			Match   interface{} `json:"match"`
// 			Lineups interface{} `json:"lineups"`
// 			Events  interface{} `json:"events"`
// 		}{
// 			Match:   match,
// 			Lineups: lineups,
// 			Events:  events,
// 		}
// 		json.NewEncoder(w).Encode(response)
// 	}
// }

// // 🔥 FUNÇÃO DE SEGUNDO PLANO (Goroutine)
// // Esta função roda sem travar a resposta HTTP. O App já recebeu a resposta
// // enquanto o servidor faz o trabalho pesado de atualizar o banco de dados.
// func syncMatchDataBackground(matchID string, league string) {
// 	utils.CustomLog("BACKGROUND", "Iniciando atualização oculta para o jogo %s", matchID)
// 	match, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

//		if err == nil {
//			database.SaveFullMatchHistory(match, lineups, events)
//			utils.CustomLog("BACKGROUND", "Atualização oculta concluída com sucesso para o jogo %s", matchID)
//		} else {
//			log.Printf("[ERRO BACKGROUND] Falha ao atualizar ESPN para o jogo %s: %v", matchID, err)
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

	// 🔥 CENÁRIO 1: O UTILIZADOR PUXOU O ECRÃ PARA ATUALIZAR (PULL TO REFRESH)
	// Aqui NÓS ESPERAMOS a ESPN responder para devolvermos os dados novos imediatamente à tela.
	if forceUpdate {
		utils.CustomLog("ESPN", "Pull to Refresh! Forçando atualização do jogo %s na ESPN...", matchID)
		match, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

		if err == nil {
			database.SaveFullMatchHistory(match, lineups, events)
		} else {
			log.Printf("[ERRO ESPN PULL TO REFRESH] %v", err)
		}

		// Devolvemos o que está na base de dados (que acabou de ser atualizado)
		historyDB, _ := database.GetFullMatchFromDB(matchID)
		json.NewEncoder(w).Encode(historyDB)
		return
	}

	// 🔥 CENÁRIO 2: ABERTURA NORMAL DA TELA (VELOCIDADE MÁXIMA)
	// Lê diretamente do Banco de Dados (Demora ~10ms a 30ms).
	historyDB, err := database.GetFullMatchFromDB(matchID)
	hasLineups := err == nil && historyDB != nil && len(historyDB.Lineups) > 0

	if hasLineups {
		utils.CustomLog("DATABASE", "Cache encontrado! Devolvendo JSON em milissegundos para o jogo %s", matchID)
		json.NewEncoder(w).Encode(historyDB)
		return
	}

	// 🔥 CENÁRIO 3: BASE DE DADOS VAZIA (FALLBACK)
	// Só entra aqui se for a primeira vez que o jogo é aberto e o Orange Pi (Worker) ainda não tiver passado por ele.
	utils.CustomLog("ESPN", "Sem escalação no DB. Buscando dados frescos na ESPN para o jogo %s...", matchID)
	match, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

	if err != nil {
		log.Printf("[ERRO ESPN FALLBACK] %v", err)
		// Se deu erro, mas a base de dados tem algo parcial (ex: o jogo, mas sem escalação), manda o parcial.
		if historyDB != nil {
			json.NewEncoder(w).Encode(historyDB)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Jogo não disponível"})
		return
	}

	// Salva a nova descoberta na base de dados
	errSave := database.SaveFullMatchHistory(match, lineups, events)
	if errSave != nil {
		utils.CustomLog("DATABASE_ERRO", "Falha ao salvar: %v", errSave)
	}

	// Busca novamente para pegar as informações unidas com os logos e nomes da nossa tabela 'teams'
	fullHistory, errFetch := database.GetFullMatchFromDB(matchID)

	if errFetch == nil && fullHistory != nil {
		json.NewEncoder(w).Encode(fullHistory)
	} else {
		// Proteção final caso a leitura do DB falhe
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

// ❌ FUNÇÃO syncMatchDataBackground REMOVIDA!
// O Render não precisa mais gastar memória a fazer isto.
// O Orange Pi é o responsável por bater na ESPN em background.
