package middlewares

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// next que registra se foi chamado e responde 401, como o JWTAuth faria sem token
func corsNext(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusUnauthorized)
	})
}

func preflight(origin string) *http.Request {
	req := httptest.NewRequest(http.MethodOptions, "/standings?league=BSA", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "authorization")
	return req
}

func TestCORSPreflightRespondidoSemPassarPelaAutenticacao(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	called := false
	rec := httptest.NewRecorder()
	CORS(corsNext(&called)).ServeHTTP(rec, preflight("https://futstatis.vercel.app"))

	if called {
		t.Fatal("o preflight não pode chegar ao handler (levaria 401 do JWTAuth)")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status esperado 204, veio %d", rec.Code)
	}
	h := rec.Header()
	if h.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Allow-Origin esperado *, veio %q", h.Get("Access-Control-Allow-Origin"))
	}
	for _, method := range []string{"GET", "POST", "DELETE"} {
		if !strings.Contains(h.Get("Access-Control-Allow-Methods"), method) {
			t.Errorf("Allow-Methods sem %s: %q", method, h.Get("Access-Control-Allow-Methods"))
		}
	}
	for _, header := range []string{"Authorization", "Content-Type"} {
		if !strings.Contains(h.Get("Access-Control-Allow-Headers"), header) {
			t.Errorf("Allow-Headers sem %s: %q", header, h.Get("Access-Control-Allow-Headers"))
		}
	}
}

func TestCORSChamadaNormalLevaHeadersMesmoComErro(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	called := false
	req := httptest.NewRequest(http.MethodGet, "/standings?league=BSA", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	CORS(corsNext(&called)).ServeHTTP(rec, req)

	if !called {
		t.Fatal("o GET precisa chegar ao handler")
	}
	// O 401 também sai com CORS: sem isso o site não consegue ler o {"error": ...}
	if rec.Code != http.StatusUnauthorized || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("esperado 401 com Allow-Origin *, veio %d e %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Expose-Headers") != "X-Cache, Retry-After" {
		t.Errorf("Expose-Headers esperado %q, veio %q", "X-Cache, Retry-After", rec.Header().Get("Access-Control-Expose-Headers"))
	}
}

func TestCORSSemOriginNaoMudaNada(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	called := false
	rec := httptest.NewRecorder()
	// O app (React Native) não manda Origin
	CORS(corsNext(&called)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/matches?league=BSA", nil))

	if !called || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("sem Origin: handler deveria rodar sem headers CORS (chamado=%v, Allow-Origin=%q)",
			called, rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSComListaDeOrigens(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", " https://futstatis.vercel.app/ , http://localhost:3000")
	handler := CORS(corsNext(new(bool)))

	// Origem da lista: devolve a própria origem (com barra e espaços tolerados na variável)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, preflight("https://futstatis.vercel.app"))
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://futstatis.vercel.app" {
		t.Errorf("Allow-Origin esperado a própria origem, veio %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Vary") != "Origin" {
		t.Errorf("Vary esperado Origin, veio %q", rec.Header().Get("Vary"))
	}

	// Origem fora da lista: 204 sem liberação, e o navegador bloqueia
	called := false
	rec = httptest.NewRecorder()
	CORS(corsNext(&called)).ServeHTTP(rec, preflight("https://site-qualquer.com"))
	if called || rec.Code != http.StatusNoContent {
		t.Errorf("preflight de origem não liberada: esperado 204 sem chamar o handler (chamado=%v, status=%d)", called, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" || rec.Header().Get("Access-Control-Allow-Methods") != "" {
		t.Errorf("origem não liberada não pode receber headers de liberação: %v", rec.Header())
	}
}

func TestCORSOptionsSemPreflightSegueParaOHandler(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	called := false
	req := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:3000") // sem Access-Control-Request-Method
	CORS(corsNext(&called)).ServeHTTP(httptest.NewRecorder(), req)
	if !called {
		t.Error("OPTIONS que não é preflight deve seguir para o handler (que responde 405)")
	}
}
