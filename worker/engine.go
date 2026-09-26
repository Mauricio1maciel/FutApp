// package worker

// import (
// 	"App-Futebol/database"
// 	"App-Futebol/services"
// 	"fmt"
// 	"time"
// )

// var lastFetched = make(map[string]time.Time)
// var lastScoreboardSync = make(map[string]time.Time) // 🔥 Controla quando lemos o Scoreboard

// func StartEngine() {
// 	fmt.Println("[WORKER_BUSCA_ESCALACAO] Motor de Background Iniciado! Executando primeira varredura...")

// 	processMatches()

// 	ticker := time.NewTicker(1 * time.Minute)
// 	defer ticker.Stop()

// 	for range ticker.C {
// 		processMatches()
// 	}
// }

// // 🔥 Esta função resolve o Problema "Tostines" (O Ovo ou a Galinha)
// func syncScoreboards() {
// 	activeLeagues := database.GetActiveLeaguesToday()

// 	for _, lg := range activeLeagues {
// 		// Só puxa o scoreboard inteiro da liga a cada 2 horas para não gastar banda
// 		if time.Since(lastScoreboardSync[lg]) > 2*time.Hour {
// 			fmt.Printf("[WORKER_BUSCA_ESCALACAO] Atualizando Mapeamento de IDs da ESPN para a liga %s...\n", lg)
// 			services.SyncESPNScoreboardForLeague(lg)
// 			lastScoreboardSync[lg] = time.Now()
// 		}
// 	}
// }

// func processMatches() {
// 	// 1. PRIMEIRO: Garante que os jogos de hoje estão mapeados na tabela 'espn_matches'
// 	syncScoreboards()

// 	// 2. SEGUNDO: Agora o nosso famoso JOIN vai encontrar o jogo perfeitamente!
// 	fmt.Println("--------------------------------------------------")
// 	fmt.Println("[WORKER_BUSCA_ESCALACAO] Buscando jogos na janela de tempo do Banco...")

// 	matchesToday, err := database.GetTodayMatches()
// 	if err != nil {
// 		fmt.Printf("[WORKER_ERRO] Erro ao consultar o banco: %v\n", err)
// 		return
// 	}

// 	fmt.Printf("[WORKER_BUSCA_ESCALACAO] Encontrados %d jogo(s) na janela de tempo.\n", len(matchesToday))

// 	nowUnix := time.Now().Unix()

// 	for _, match := range matchesToday {
// 		timeToMatch := match.MatchTimeUnix - nowUnix

// 		// Regra de Proteção contra Spam
// 		last, exists := lastFetched[match.IDEvent]
// 		cooldown := 5 * time.Minute
// 		if match.Status == "in" {
// 			cooldown = 1 * time.Minute
// 		}
// 		if exists && time.Since(last) < cooldown {
// 			continue
// 		}

// 		if timeToMatch <= 2100 && match.Status == "pre" {
// 			fmt.Printf("[WORKER_BUSCA_ESCALACAO] -> [PRÉ-JOGO] Acionando gatilho da ESPN para o jogo %s!\n", match.IDEvent)
// 			lastFetched[match.IDEvent] = time.Now()
// 			go fetchAndSave(match.IDEvent, match.League)
// 		} else if match.Status == "in" {
// 			fmt.Printf("[WORKER_BUSCA_ESCALACAO] -> [AO VIVO] Acionando gatilho da ESPN para o jogo %s!\n", match.IDEvent)
// 			lastFetched[match.IDEvent] = time.Now()
// 			go fetchAndSave(match.IDEvent, match.League)
// 		}
// 	}
// 	fmt.Println("--------------------------------------------------")
// }

// func fetchAndSave(matchID string, league string) {
// 	matchData, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

//		if err == nil {
//			database.SaveFullMatchHistory(matchData, lineups, events)
//			fmt.Printf("[WORKER_BUSCA_ESCALACAO] ✅ Escalações e eventos salvos no Banco! (Jogo %s)\n", matchID)
//		} else {
//			fmt.Printf("[WORKER_ERRO] ❌ Falha na sincronização ESPN (Jogo %s): %v\n", matchID, err)
//		}
//	}
package worker

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"log"
	"time"
)

var lastFetched = make(map[string]time.Time)
var lastScoreboardSync = make(map[string]time.Time) // 🔥 Controla quando lemos o Scoreboard

func StartEngine() {
	log.Println("[WORKER_BUSCA_ESCALACAO] Motor de Background Iniciado! Executando primeira varredura...")

	processMatches()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		processMatches()
	}
}

// 🔥 Esta função resolve o Problema "Tostines" (O Ovo ou a Galinha)
func syncScoreboards() {
	activeLeagues := database.GetActiveLeaguesToday()

	for _, lg := range activeLeagues {
		// Só puxa o scoreboard inteiro da liga a cada 2 horas para não gastar banda
		if time.Since(lastScoreboardSync[lg]) > 2*time.Hour {
			log.Printf("[WORKER_BUSCA_ESCALACAO] Atualizando Mapeamento de IDs da ESPN para a liga %s...", lg)
			services.SyncESPNScoreboardForLeague(lg)
			lastScoreboardSync[lg] = time.Now()
		}
	}
}

func processMatches() {
	// 1. PRIMEIRO: Garante que os jogos de hoje estão mapeados na tabela 'espn_matches'
	syncScoreboards()

	// 2. SEGUNDO: Agora o nosso famoso JOIN vai encontrar o jogo perfeitamente!
	log.Println("--------------------------------------------------")
	log.Println("[WORKER_BUSCA_ESCALACAO] Buscando jogos na janela de tempo do Banco...")

	matchesToday, err := database.GetTodayMatches()
	if err != nil {
		log.Printf("[WORKER_ERRO] Erro ao consultar o banco: %v", err)
		return
	}

	log.Printf("[WORKER_BUSCA_ESCALACAO] Encontrados %d jogo(s) na janela de tempo.", len(matchesToday))

	nowUnix := time.Now().Unix()

	for _, match := range matchesToday {
		timeToMatch := match.MatchTimeUnix - nowUnix

		// Regra de Proteção contra Spam
		last, exists := lastFetched[match.IDEvent]
		cooldown := 5 * time.Minute
		if match.Status == "in" {
			cooldown = 1 * time.Minute
		}
		if exists && time.Since(last) < cooldown {
			continue
		}

		if timeToMatch <= 2100 && match.Status == "pre" {
			log.Printf("[WORKER_BUSCA_ESCALACAO] -> [PRÉ-JOGO] Acionando gatilho da ESPN para o jogo %s!", match.IDEvent)
			lastFetched[match.IDEvent] = time.Now()
			go fetchAndSave(match.IDEvent, match.League)
		} else if match.Status == "in" {
			log.Printf("[WORKER_BUSCA_ESCALACAO] -> [AO VIVO] Acionando gatilho da ESPN para o jogo %s!", match.IDEvent)
			lastFetched[match.IDEvent] = time.Now()
			go fetchAndSave(match.IDEvent, match.League)
		}
	}
	log.Println("--------------------------------------------------")
}

func fetchAndSave(matchID string, league string) {
	matchData, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

	if err == nil {
		database.SaveFullMatchHistory(matchData, lineups, events)
		log.Printf("[WORKER_BUSCA_ESCALACAO] ✅ Escalações e eventos salvos no Banco! (Jogo %s)", matchID)
	} else {
		log.Printf("[WORKER_ERRO] ❌ Falha na sincronização ESPN (Jogo %s): %v", matchID, err)
	}
}
