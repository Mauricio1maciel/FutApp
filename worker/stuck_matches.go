package worker

import (
	"App-Futebol/services"
	"App-Futebol/utils"
	"context"
	"time"
)

// loopStuckMatches procura, a cada 30 minutos, jogos que já deviam ter acabado e
// continuam sem resultado (services.CheckStuckMatches). A primeira volta espera 10
// minutos, para as sincronizações de quando o worker sobe terminarem antes.
func loopStuckMatches() {
	time.Sleep(10 * time.Minute)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if err := services.CheckStuckMatches(ctx); err != nil {
			utils.CustomLog("JOBS", "Erro ao procurar jogos sem resultado: %v", err)
		}
		cancel()
		time.Sleep(30 * time.Minute)
	}
}
