package middlewares

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// countingHandler responde status/corpo fixos e conta quantas vezes foi chamado
func countingHandler(status int, body string) (http.HandlerFunc, *int) {
	calls := 0
	return func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}, &calls
}

func get(h http.HandlerFunc, target string, claims *utils.Claims) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), claimsKey, claims))
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestCacheGuardaEDevolveIgual(t *testing.T) {
	next, calls := countingHandler(200, `[{"id":1}]`)
	h := CacheFor(time.Minute, next)

	first := get(h, "/cache-teste-1?league=BSA&round=30", nil)
	second := get(h, "/cache-teste-1?round=30&league=BSA", nil) // mesmos parâmetros, outra ordem

	if *calls != 1 {
		t.Fatalf("o handler deveria rodar 1 vez, rodou %d", *calls)
	}
	if first.Header().Get("X-Cache") != "MISS" || second.Header().Get("X-Cache") != "HIT" {
		t.Errorf("X-Cache esperado MISS e HIT, veio %q e %q", first.Header().Get("X-Cache"), second.Header().Get("X-Cache"))
	}
	if second.Body.String() != `[{"id":1}]` || second.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("resposta do cache diferente da original: %q (%s)", second.Body.String(), second.Header().Get("Content-Type"))
	}

	get(h, "/cache-teste-1?league=PL&round=30", nil) // outra liga: outra chave
	if *calls != 2 {
		t.Errorf("parâmetro diferente deveria ir ao handler, chamadas = %d", *calls)
	}
}

func TestCacheNaoGuardaErro(t *testing.T) {
	next, calls := countingHandler(500, `{"error":"banco fora"}`)
	h := CacheFor(time.Minute, next)
	get(h, "/cache-teste-2", nil)
	get(h, "/cache-teste-2", nil)
	if *calls != 2 {
		t.Errorf("erro não pode ficar no cache: handler rodou %d vez(es)", *calls)
	}
}

func TestCacheAdminEPostPassamDireto(t *testing.T) {
	next, calls := countingHandler(200, `[]`)
	h := CacheFor(time.Minute, next)
	admin := &utils.Claims{Role: models.RoleAdmin}

	get(h, "/cache-teste-3?update=true", admin)
	get(h, "/cache-teste-3?update=true", admin)
	if *calls != 2 {
		t.Errorf("admin deveria sempre receber dado fresco: handler rodou %d vez(es)", *calls)
	}

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/cache-teste-3", nil))
	if *calls != 3 || rec.Header().Get("X-Cache") != "BYPASS" {
		t.Errorf("POST não pode usar o cache (chamadas = %d, X-Cache = %q)", *calls, rec.Header().Get("X-Cache"))
	}
}

func TestCacheExpira(t *testing.T) {
	next, calls := countingHandler(200, `[]`)
	h := CacheFor(50*time.Millisecond, next)
	get(h, "/cache-teste-4", nil)
	time.Sleep(80 * time.Millisecond)
	get(h, "/cache-teste-4", nil)
	if *calls != 2 {
		t.Errorf("depois do prazo, o handler deveria rodar de novo: rodou %d vez(es)", *calls)
	}
}
