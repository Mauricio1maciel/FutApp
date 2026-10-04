package handlers

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"context"
	"net/http"
	"time"
)

const healthTimeout = 2 * time.Second

// HealthHandler é usado pelo health check do Render (sem token).
// Responde 200 se a API e o banco estão de pé, 503 se o banco não responde.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthTimeout)
	defer cancel()

	// O lib/pq não interrompe uma conexão travada quando o contexto vence, então o
	// ping roda em paralelo e o health check responde no prazo de qualquer jeito
	result := make(chan error, 1)
	go func() { result <- database.DB.PingContext(ctx) }()

	var err error
	select {
	case err = <-result:
	case <-time.After(healthTimeout):
		err = context.DeadlineExceeded
	}

	if err != nil {
		utils.CustomLog("HTTP", "Health check falhou: banco não respondeu: %v", err)
		utils.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "erro", "database": "indisponível"})
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "ok"})
}
