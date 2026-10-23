package worker

import (
	"App-Futebol/services"
	"App-Futebol/utils"
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// Um pânico (panic) numa goroutine derruba o programa inteiro: uma resposta inesperada da
// ESPN num único jogo pararia o placar ao vivo, os avisos e o backup juntos, até alguém
// reiniciar o worker. Cada laço roda sob supervise, que registra o pânico e recomeça o
// laço; cada volta do placar ao vivo e cada busca de jogo rodam em protect.

// lastPanic guarda o último pânico que o sinal de vida (healthcheck) ainda não contou
var lastPanic atomic.Value // string

// Espera antes de recomeçar um laço que entrou em pânico. Sem ela, um laço que quebra
// sempre encheria o log. Variável para os testes
var superviseRestartDelay = time.Minute

// protect roda fn. Se ela entrar em pânico, registra o erro e a pilha, avisa o admin e
// devolve true, em vez de derrubar o worker
func protect(name string, fn func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			msg := fmt.Sprintf("pânico em %s: %v", name, r)
			utils.CustomLog("WORKER_ERRO", "%s\n%s", msg, debug.Stack())
			lastPanic.Store(msg)
			notifyPanic(msg)
		}
	}()
	fn()
	return false
}

// supervise roda um laço que não deveria terminar e o recomeça se ele entrar em pânico.
// Se ele terminar sozinho (ex: o sinal de vida sem HEALTHCHECK_URL), termina também.
func supervise(name string, loop func()) {
	for protect(name, loop) {
		time.Sleep(superviseRestartDelay)
	}
}

var (
	panicNotifyMu   sync.Mutex
	lastPanicNotify time.Time
)

// panicNotifier manda o aviso ao admin. Variável para os testes
var panicNotifier = func(msg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	services.NotifyAdmins(ctx, "⚠️ Erro no worker", msg+". O laço foi reiniciado; detalhes no log.")
}

// notifyPanic avisa o admin por push, no máximo uma vez por hora: um laço que quebra a
// cada minuto não pode virar um aviso por minuto
func notifyPanic(msg string) {
	panicNotifyMu.Lock()
	if !lastPanicNotify.IsZero() && time.Since(lastPanicNotify) < time.Hour {
		panicNotifyMu.Unlock()
		return
	}
	lastPanicNotify = time.Now()
	panicNotifyMu.Unlock()

	notify := panicNotifier
	go func() {
		defer func() { recover() }() // o aviso nunca pode derrubar o worker
		notify(msg)
	}()
}

// takeLastPanic devolve o último pânico ainda não contado ao sinal de vida e o esquece
func takeLastPanic() string {
	msg, _ := lastPanic.Swap("").(string)
	return msg
}
