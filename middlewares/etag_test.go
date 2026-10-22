package middlewares

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func etagHandler(body *string, status int) http.Handler {
	return ETag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(*body))
	}))
}

func etagGet(h http.Handler, path, ifNoneMatch string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestETagNadaMudouVem304SemCorpo(t *testing.T) {
	body := `{"matches":[1,2,3]}`
	h := etagHandler(&body, http.StatusOK)

	first := etagGet(h, "/matches?league=BSA", "")
	tag := first.Header().Get("ETag")
	if first.Code != http.StatusOK || first.Body.String() != body {
		t.Fatalf("1º pedido: esperado 200 com o corpo, veio %d %q", first.Code, first.Body.String())
	}
	if !strings.HasPrefix(tag, `W/"`) {
		t.Fatalf("etiqueta fraca esperada (sobrevive à compressão da Cloudflare), veio %q", tag)
	}
	if cc := first.Header().Get("Cache-Control"); cc != "private, no-cache" {
		t.Errorf("Cache-Control esperado %q, veio %q", "private, no-cache", cc)
	}

	// Mesma etiqueta, também na forma forte e no meio de uma lista: 304 sem corpo
	for _, inm := range []string{tag, strings.TrimPrefix(tag, "W/"), `W/"outra", ` + tag} {
		rec := etagGet(h, "/matches?league=BSA", inm)
		if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
			t.Errorf("If-None-Match %s: esperado 304 sem corpo, veio %d com %d bytes", inm, rec.Code, rec.Body.Len())
		}
		if rec.Header().Get("ETag") != tag {
			t.Errorf("o 304 precisa repetir a etiqueta, veio %q", rec.Header().Get("ETag"))
		}
	}

	// Mudou: corpo novo, etiqueta nova
	body = `{"matches":[1,2,3,4]}`
	rec := etagGet(h, "/matches?league=BSA", tag)
	if rec.Code != http.StatusOK || rec.Body.String() != body || rec.Header().Get("ETag") == tag {
		t.Errorf("depois da mudança: esperado 200 com corpo e etiqueta novos, veio %d %q %q",
			rec.Code, rec.Body.String(), rec.Header().Get("ETag"))
	}
}

func TestETagSoNasLeiturasQueDeramCerto(t *testing.T) {
	body := `{"error":"token inválido"}`
	rec := etagGet(etagHandler(&body, http.StatusUnauthorized), "/matches", "")
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != body || rec.Header().Get("ETag") != "" {
		t.Errorf("erro deveria passar igual e sem etiqueta: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("ETag"))
	}

	ok := `{"ok":true}`
	h := etagHandler(&ok, http.StatusOK)
	post := httptest.NewRecorder()
	h.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/push/subscriptions", nil))
	if post.Header().Get("ETag") != "" || post.Body.String() != ok {
		t.Errorf("POST não leva etiqueta: %q %q", post.Header().Get("ETag"), post.Body.String())
	}
	if admin := etagGet(h, "/admin/db-size", ""); admin.Header().Get("ETag") != "" || admin.Body.String() != ok {
		t.Errorf("rota de admin não leva etiqueta: %q", admin.Header().Get("ETag"))
	}
}
