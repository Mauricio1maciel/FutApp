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
		panico       string
		falha        bool
	}{
		{"tudo rodando", now, recente, recente, nil, "", false},
		{"banco fora", now, recente, recente, errors.New("timeout"), "", true},
		{"placar ao vivo parado", now, velho, recente, nil, "", true},
		{"football-data parada", now, recente, velho, nil, "", true},
		{"acabou de subir (laços sem volta ainda)", start.Add(2 * time.Minute), 0, 0, nil, "", false},
		{"pânico num laço (recomeçado)", now, recente, recente, nil, "pânico em backup: x", true},
	}
	for _, c := range casos {
		if got := healthProblem(c.now, start, c.engine, c.data, c.dbErr, c.panico); (got != "") != c.falha {
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

func TestFetchWebPushKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/push/web-key" {
			w.WriteHeader(404)
			return
		}
		w.Write([]byte(`{"public_key": "BChaveDaApi"}`))
	}))
	defer srv.Close()
	if key, err := fetchWebPushKey(srv.Client(), srv.URL); err != nil || key != "BChaveDaApi" {
		t.Errorf("chave da API: veio %q (%v)", key, err)
	}

	off := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer off.Close()
	if _, err := fetchWebPushKey(off.Client(), off.URL); err == nil {
		t.Error("API sem as chaves (503) deveria dar erro")
	}
}
