package worker

import (
	"App-Futebol/utils"
	"net/http"
	"os"
	"strings"
	"time"
)

// loopKeepAPIAwake mantém a API do Render acordada.
//
// No plano grátis, o Render desliga a API depois de 15 minutos sem pedidos, e o primeiro
// pedido seguinte espera ~50 segundos ela voltar: no 4G, o app parece quebrado. O worker
// fica ligado o tempo todo, então chama o /health da API a cada 10 minutos.
//
// Liga com API_PUBLIC_URL (ex: https://futapp-r38z.onrender.com) no .env do worker.
func loopKeepAPIAwake() {
	base := strings.TrimRight(os.Getenv("API_PUBLIC_URL"), "/")
	if base == "" {
		return
	}
	// Folga para o caso de pegar a API acordando (~50s)
	client := &http.Client{Timeout: 90 * time.Second}

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		start := time.Now()
		resp, err := client.Get(base + "/health")
		if err != nil {
			utils.CustomLog("JOBS", "API não respondeu ao /health: %v", err)
			continue
		}
		resp.Body.Close()
		if took := time.Since(start); took > 10*time.Second || resp.StatusCode != http.StatusOK {
			utils.CustomLog("JOBS", "API acordada: /health %d em %s", resp.StatusCode, took.Round(time.Second))
		}
	}
}
