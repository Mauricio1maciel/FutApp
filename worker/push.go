package worker

import (
	"App-Futebol/services"
	"context"
	"time"
)

// loopPushNotifications manda os avisos da fila (push_outbox). Os avisos são anotados
// no banco quando o placar, o status ou a escalação de um jogo seguido muda.
func loopPushNotifications() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		services.ProcessPushOutbox(context.Background())
	}
}
