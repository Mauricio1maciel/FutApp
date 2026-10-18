package worker

import (
	"App-Futebol/services"
	"App-Futebol/utils"
	"context"
	"time"
)

// loopDatabaseSize mede o banco logo depois de o worker subir e, depois, todo dia ao
// meio-dia (para o aviso não chegar de madrugada). O push só sai quando o banco sobe de
// nível (services.CheckDatabaseSize), então medir de novo não repete o aviso.
func loopDatabaseSize() {
	time.Sleep(time.Minute)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		if err := services.CheckDatabaseSize(ctx); err != nil {
			utils.CustomLog("JOBS", "Erro ao medir o tamanho do banco: %v", err)
		}
		cancel()

		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		time.Sleep(time.Until(next))
	}
}
