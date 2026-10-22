package middlewares

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientIP(t *testing.T) {
	req := func(remote string, headers map[string]string) *http.Request {
		r := httptest.NewRequest("GET", "/matches", nil)
		r.RemoteAddr = remote
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		return r
	}
	casos := []struct {
		nome     string
		r        *http.Request
		esperado string
	}{
		{"Cloudflare vence o X-Forwarded-For (que o cliente pode inventar)",
			req("10.0.0.5:1234", map[string]string{"CF-Connecting-IP": "200.1.2.3", "X-Forwarded-For": "1.1.1.1"}), "200.1.2.3"},
		{"sem Cloudflare: primeiro do X-Forwarded-For",
			req("10.0.0.5:1234", map[string]string{"X-Forwarded-For": "200.1.2.3, 10.0.0.9"}), "200.1.2.3"},
		{"CF-Connecting-IP inválido é ignorado",
			req("10.0.0.5:1234", map[string]string{"CF-Connecting-IP": "lixo", "X-Forwarded-For": "200.1.2.3"}), "200.1.2.3"},
		{"sem proxy: endereço da conexão", req("200.9.9.9:5555", nil), "200.9.9.9"},
	}
	for _, c := range casos {
		if got := ClientIP(c.r); got != c.esperado {
			t.Errorf("%s: esperado %s, veio %s", c.nome, c.esperado, got)
		}
	}
}

func TestRateLimiterBalde(t *testing.T) {
	agora := time.Date(2026, 10, 22, 20, 0, 0, 0, time.UTC)
	l := newRateLimiter()
	l.now = func() time.Time { return agora }
	c := rateClass{"teste", 60, 3} // 1 por segundo, 3 seguidos

	for i := 0; i < 3; i++ {
		if ok, _, _, _ := l.allow("k", c); !ok {
			t.Fatalf("pedido %d deveria passar", i+1)
		}
	}
	ok, _, wait, primeiro := l.allow("k", c)
	if ok || wait != time.Second || !primeiro {
		t.Errorf("4º pedido: deveria esperar 1 s (e ser o primeiro recusado), veio ok=%v espera=%s primeiro=%v", ok, wait, primeiro)
	}
	if _, _, _, primeiro := l.allow("k", c); primeiro {
		t.Error("o 5º recusado seguido não é mais o primeiro (não loga de novo)")
	}
	if ok, _, _, _ := l.allow("outro", c); !ok {
		t.Error("outro endereço tem balde próprio")
	}
	agora = agora.Add(time.Second)
	if ok, _, _, _ := l.allow("k", c); !ok {
		t.Error("depois de 1 s, recarrega 1 pedido")
	}

	agora = agora.Add(11 * time.Minute)
	l.cleanup()
	if len(l.buckets) != 0 {
		t.Errorf("baldes parados há mais de 10 min deveriam sair, sobraram %d", len(l.buckets))
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter = newRateLimiter()
	t.Cleanup(func() { limiter = newRateLimiter() })
	h := RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	call := func(method, path, ip string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("CF-Connecting-IP", ip)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}

	// /auth/guest: 10 seguidos (criar token é o que um script repetiria)
	for i := 0; i < 10; i++ {
		if rec := call("POST", "/auth/guest", "200.1.1.1"); rec.Code != 200 {
			t.Fatalf("pedido %d de token: veio %d", i+1, rec.Code)
		}
	}
	rec := call("POST", "/auth/guest", "200.1.1.1")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" ||
		!strings.Contains(rec.Body.String(), "Muitos pedidos") {
		t.Errorf("11º token: esperado 429 com Retry-After, veio %d %q (%s)", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
	}
	if rec := call("POST", "/auth/guest", "200.2.2.2"); rec.Code != 200 {
		t.Errorf("outro endereço não pode ser afetado, veio %d", rec.Code)
	}
	if rec := call("GET", "/matches", "200.1.1.1"); rec.Code != 200 {
		t.Errorf("limite de token não pode bloquear as leituras do mesmo endereço, veio %d", rec.Code)
	}

	// Sem limite: /health, preflight e endereço interno (configuração do proxy mudou)
	for i := 0; i < 300; i++ {
		for _, c := range []struct{ method, path, ip string }{
			{"GET", "/health", "200.3.3.3"}, {"OPTIONS", "/search", "200.3.3.3"}, {"GET", "/search", "10.0.0.1"},
		} {
			if rec := call(c.method, c.path, c.ip); rec.Code != 200 {
				t.Fatalf("%s %s (%s) não deveria ter limite, veio %d", c.method, c.path, c.ip, rec.Code)
			}
		}
	}

	// Rotas pesadas: 30 seguidos
	var ultimo int
	for i := 0; i < 31; i++ {
		ultimo = call("GET", "/search?q=fla", "200.4.4.4").Code
	}
	if ultimo != http.StatusTooManyRequests {
		t.Errorf("31ª busca seguida deveria dar 429, veio %d", ultimo)
	}
	// As chances (simulação) também são pesadas, num balde só delas por endereço
	for i := 0; i < 31; i++ {
		ultimo = call("GET", "/standings/chances?league=BSA", "200.5.5.5").Code
	}
	if ultimo != http.StatusTooManyRequests {
		t.Errorf("31º pedido seguido de chances deveria dar 429, veio %d", ultimo)
	}

	// RATE_LIMIT=off desliga
	t.Setenv("RATE_LIMIT", "off")
	if rec := call("GET", "/search?q=fla", "200.4.4.4"); rec.Code != 200 {
		t.Errorf("com RATE_LIMIT=off não há limite, veio %d", rec.Code)
	}
}
