package worker

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Os avisos ao admin vão para um canal, em vez do push de verdade
func avisosDePanico(t *testing.T) chan string {
	t.Helper()
	avisos := make(chan string, 10)
	original := panicNotifier
	panicNotifier = func(msg string) { avisos <- msg }
	panicNotifyMu.Lock()
	lastPanicNotify = time.Time{}
	panicNotifyMu.Unlock()
	t.Cleanup(func() { panicNotifier = original })
	return avisos
}

func TestProtectSegueDepoisDoPanico(t *testing.T) {
	avisos := avisosDePanico(t)
	takeLastPanic()

	if protect("teste", func() {}) {
		t.Error("sem pânico, protect deveria devolver false")
	}
	if !protect("placar ao vivo", func() { var m map[string]int; m["x"]++ }) {
		t.Fatal("com pânico, protect deveria devolver true")
	}
	if msg := takeLastPanic(); !strings.Contains(msg, "pânico em placar ao vivo") {
		t.Errorf("o sinal de vida deveria receber o pânico, veio %q", msg)
	}
	if takeLastPanic() != "" {
		t.Error("o pânico deveria ser contado uma vez só ao sinal de vida")
	}
	select {
	case msg := <-avisos:
		if !strings.Contains(msg, "placar ao vivo") {
			t.Errorf("aviso ao admin: %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("o admin deveria ser avisado do pânico")
	}

	// Outro pânico na mesma hora: registrado, mas sem outro aviso
	protect("backup", func() { panic("de novo") })
	select {
	case msg := <-avisos:
		t.Errorf("no máximo um aviso por hora, veio outro: %q", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSuperviseRecomecaOLaco(t *testing.T) {
	avisosDePanico(t)
	original := superviseRestartDelay
	superviseRestartDelay = 0
	t.Cleanup(func() { superviseRestartDelay = original })

	voltas := 0
	supervise("teste", func() {
		voltas++
		if voltas < 3 {
			panic("quebrou")
		}
	})
	if voltas != 3 {
		t.Errorf("o laço deveria recomeçar até terminar sozinho: %d voltas, esperado 3", voltas)
	}
}

func TestStartFetchNaoDuplicaABuscaDoMesmoJogo(t *testing.T) {
	avisosDePanico(t)
	original := fetchMatch
	t.Cleanup(func() { fetchMatch = original })

	libera := make(chan struct{})
	var buscas atomic.Int32
	fetchMatch = func(matchID, league string) {
		buscas.Add(1)
		<-libera
		if matchID == "quebra" {
			panic("resposta estranha da ESPN")
		}
	}

	if !startFetch("401", "BSA") {
		t.Fatal("a primeira busca do jogo deveria começar")
	}
	if startFetch("401", "BSA") {
		t.Error("com a busca anterior ainda rodando, não deveria começar outra do mesmo jogo")
	}
	if !startFetch("quebra", "BSA") {
		t.Fatal("outro jogo deveria buscar normalmente")
	}
	close(libera)

	// As duas terminam (uma com pânico, sem derrubar o teste) e liberam o jogo
	prazo := time.Now().Add(time.Second)
	for {
		_, a := fetching.Load("401")
		_, b := fetching.Load("quebra")
		if !a && !b {
			break
		}
		if time.Now().After(prazo) {
			t.Fatal("as buscas deveriam terminar e liberar os jogos")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if buscas.Load() != 2 {
		t.Errorf("esperado 2 buscas, veio %d", buscas.Load())
	}
	if !startFetch("quebra", "BSA") {
		t.Error("depois do pânico, o jogo deveria poder ser buscado de novo")
	}
	for prazo := time.Now().Add(time.Second); time.Now().Before(prazo); time.Sleep(5 * time.Millisecond) {
		if _, ok := fetching.Load("quebra"); !ok {
			return
		}
	}
	t.Error("a última busca deveria terminar")
}
