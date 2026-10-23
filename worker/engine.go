package worker

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"context"
	"log"
	"sync"
	"time"
)

var lastFetched = make(map[string]time.Time)
var lastScoreboardSync = make(map[string]time.Time) // 🔥 Controla quando lemos o Scoreboard

func StartEngine() {
	log.Println("[WORKER_BUSCA_ESCALACAO] Motor de Background Iniciado! Executando primeira varredura...")

	startDataJobs()

	// Volta protegida: um pânico numa volta não para as próximas. O sinal de vida só conta
	// a volta que terminou (com pânico em toda volta, ele acusa o laço parado)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for ; ; <-ticker.C {
		if !protect("placar ao vivo", processMatches) {
			lastEngineRun.Store(time.Now().Unix())
		}
	}
}

// Buscas de jogo em andamento. Com a ESPN ou o banco lentos, a busca de um jogo pode
// passar de um minuto, e a volta seguinte começaria outra do mesmo jogo: duas transações
// regravando a mesma escalação ao mesmo tempo
var fetching sync.Map // IDEvent → struct{}

// fetchMatch busca e grava um jogo. Variável para os testes
var fetchMatch = fetchAndSave

// startFetch busca o jogo em segundo plano, protegido, se ele já não estiver sendo buscado
func startFetch(matchID, league string) bool {
	if _, busy := fetching.LoadOrStore(matchID, struct{}{}); busy {
		return false
	}
	fetch := fetchMatch
	go func() {
		defer fetching.Delete(matchID)
		protect("busca do jogo "+matchID+" na ESPN", func() { fetch(matchID, league) })
	}()
	return true
}

// 🔥 Esta função resolve o Problema "Tostines" (O Ovo ou a Galinha)
func syncScoreboards(ctx context.Context) {
	activeLeagues := database.GetActiveLeaguesToday(ctx)

	for _, lg := range activeLeagues {
		// Só puxa o scoreboard inteiro da liga a cada 2 horas para não gastar banda
		if time.Since(lastScoreboardSync[lg]) > 2*time.Hour {
			log.Printf("[WORKER_BUSCA_ESCALACAO] Atualizando Mapeamento de IDs da ESPN para a liga %s...", lg)
			services.SyncESPNScoreboardForLeague(ctx, lg)
			lastScoreboardSync[lg] = time.Now()
		}
	}
}

func processMatches() {
	ctx := context.Background()

	// 1. PRIMEIRO: Garante que os jogos de hoje estão mapeados na tabela 'espn_matches'
	syncScoreboards(ctx)

	// 2. SEGUNDO: Agora o nosso famoso JOIN vai encontrar o jogo perfeitamente!
	log.Println("--------------------------------------------------")
	log.Println("[WORKER_BUSCA_ESCALACAO] Buscando jogos na janela de tempo do Banco...")

	matchesToday, err := database.GetTodayMatches(ctx)
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
			// Menos que a volta do ticker (1 minuto): com 1 minuto exato, uma volta que
			// chegasse uns milissegundos adiantada pulava o jogo, e o placar (e o aviso
			// de gol) atrasava mais um minuto
			cooldown = 50 * time.Second
		}
		if exists && time.Since(last) < cooldown {
			continue
		}

		if timeToMatch <= 2100 && match.Status == "pre" {
			log.Printf("[WORKER_BUSCA_ESCALACAO] -> [PRÉ-JOGO] Acionando gatilho da ESPN para o jogo %s!", match.IDEvent)
			lastFetched[match.IDEvent] = time.Now()
			startFetch(match.IDEvent, match.League)
		} else if match.Status == "in" {
			log.Printf("[WORKER_BUSCA_ESCALACAO] -> [AO VIVO] Acionando gatilho da ESPN para o jogo %s!", match.IDEvent)
			lastFetched[match.IDEvent] = time.Now()
			startFetch(match.IDEvent, match.League)
		}
	}
	log.Println("--------------------------------------------------")

	// Jogos de dias atrás não voltam: sem isso, o mapa crescia para sempre
	for id, last := range lastFetched {
		if time.Since(last) > 24*time.Hour {
			delete(lastFetched, id)
		}
	}
}

func fetchAndSave(matchID string, league string) {
	matchData, lineups, events, err := services.FetchAndParseESPNMatch(matchID, league)

	if err == nil {
		database.SaveFullMatchHistory(context.Background(), matchData, lineups, events)
		log.Printf("[WORKER_BUSCA_ESCALACAO] ✅ Escalações e eventos salvos no Banco! (Jogo %s)", matchID)
	} else {
		log.Printf("[WORKER_ERRO] ❌ Falha na sincronização ESPN (Jogo %s): %v", matchID, err)
	}
}
