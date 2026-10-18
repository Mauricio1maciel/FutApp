package worker

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthProblem(t *testing.T) {
	start := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	now := start.Add(30 * time.Minute)
	recente := now.Add(-time.Minute).Unix()
	velho := now.Add(-11 * time.Minute).Unix()

	casos := []struct {
		nome         string
		now          time.Time
		engine, data int64
		dbErr        error
		falha        bool
	}{
		{"tudo rodando", now, recente, recente, nil, false},
		{"banco fora", now, recente, recente, errors.New("timeout"), true},
		{"placar ao vivo parado", now, velho, recente, nil, true},
		{"football-data parada", now, recente, velho, nil, true},
		{"acabou de subir (laços sem volta ainda)", start.Add(2 * time.Minute), 0, 0, nil, false},
	}
	for _, c := range casos {
		if got := healthProblem(c.now, start, c.engine, c.data, c.dbErr); (got != "") != c.falha {
			t.Errorf("%s: esperado falha=%v, veio %q", c.nome, c.falha, got)
		}
	}
}

func TestSendHealthcheck(t *testing.T) {
	var path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		path, body = r.URL.Path, string(b)
	}))
	defer srv.Close()

	if err := sendHealthcheck(srv.Client(), srv.URL+"/abc", ""); err != nil || path != "/abc" || body != "ok" {
		t.Errorf("sinal ok: veio %q %q (%v)", path, body, err)
	}
	if err := sendHealthcheck(srv.Client(), srv.URL+"/abc", "banco fora"); err != nil || path != "/abc/fail" || body != "banco fora" {
		t.Errorf("sinal de falha: veio %q %q (%v)", path, body, err)
	}
}
