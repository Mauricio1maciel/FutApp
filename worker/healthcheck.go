package worker

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// Última volta de cada laço principal (Unix): o sinal de vida só sai "ok" se os placares
// estão mesmo sendo atualizados, não só se o programa está aberto
var (
	lastEngineRun atomic.Int64 // placar ao vivo e escalações (ESPN)
	lastDataRun   atomic.Int64 // jogos da football-data
)

// loopHealthcheck manda um sinal de vida a um monitor externo (ex: healthchecks.io) a
// cada 5 minutos. Se o Orange Pi travar, ficar sem energia ou sem internet, os sinais
// param e o monitor avisa por e-mail ou Telegram. Banco fora do ar ou laço parado mandam
// o sinal de falha (/fail), com o motivo. O alerta de tamanho do banco não cobre isso:
// ele roda no próprio worker.
//
// Liga com HEALTHCHECK_URL (ex: https://hc-ping.com/<uuid>) no .env do worker.
func loopHealthcheck() {
	url := strings.TrimRight(os.Getenv("HEALTHCHECK_URL"), "/")
	if url == "" {
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}
	start := time.Now()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for ; ; <-ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		dbErr := database.DB.PingContext(ctx)
		cancel()

		problem := healthProblem(time.Now(), start, lastEngineRun.Load(), lastDataRun.Load(), dbErr, takeLastPanic())
		if err := sendHealthcheck(client, url, problem); err != nil {
			utils.CustomLog("JOBS", "Monitor externo não respondeu: %v", err)
		}
	}
}

// healthProblem devolve o motivo da falha ("" = tudo certo). Os laços rodam a cada
// minuto; 10 minutos sem volta é travamento. Logo depois de subir, os laços ainda não
// tiveram tempo de dar a primeira volta. lastPanic é o último pânico desde o sinal
// anterior: o laço foi recomeçado, mas alguém precisa olhar o log.
func healthProblem(now, start time.Time, lastEngine, lastData int64, dbErr error, lastPanic string) string {
	if dbErr != nil {
		return "banco de dados fora do ar: " + dbErr.Error()
	}
	if lastPanic != "" {
		return lastPanic + " (laço recomeçado; detalhes no log)"
	}
	if now.Sub(start) < 10*time.Minute {
		return ""
	}
	stale := func(last int64) bool { return now.Sub(time.Unix(last, 0)) > 10*time.Minute }
	switch {
	case stale(lastEngine):
		return "laço do placar ao vivo (ESPN) parado há mais de 10 minutos"
	case stale(lastData):
		return "laço da football-data parado há mais de 10 minutos"
	}
	return ""
}

// sendHealthcheck manda o sinal (POST com o motivo no corpo, que aparece no aviso)
func sendHealthcheck(client *http.Client, url, problem string) error {
	target, body := url, "ok"
	if problem != "" {
		target, body = url+"/fail", problem
		utils.CustomLog("JOBS", "Sinal de falha ao monitor: %s", problem)
	}
	resp, err := client.Post(target, "text/plain; charset=utf-8", strings.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("monitor respondeu %s", resp.Status)
	}
	return nil
}
