package middlewares

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"net/http"
	"strings"
)

// Etiqueta (ETag) nas respostas de leitura. Quem pede manda de volta a etiqueta da última
// resposta em If-None-Match e, se nada mudou, recebe 304 sem corpo. A tela inicial do app
// pede ~16 listas a cada ~12s e quase sempre recebe o mesmo de antes: no 4G, esse corpo
// repetido é a maior parte do tráfego.
//
// A etiqueta é fraca (W/) porque a Cloudflare, na frente do Render, comprime a resposta,
// e etiqueta forte não sobrevive à compressão. "private, no-cache" deixa o navegador
// guardar a resposta, sempre conferindo antes de usar, e nenhum cache no caminho guardá-la.
//
// As rotas de admin ficam de fora: algumas demoram e não precisam esperar o fim para
// responder.

// bufferedWriter segura a resposta inteira: a etiqueta depende do corpo todo
type bufferedWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (b *bufferedWriter) WriteHeader(code int) {
	if b.status == 0 {
		b.status = code
	}
}

func (b *bufferedWriter) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(p)
}

func ETag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || strings.HasPrefix(r.URL.Path, "/admin/") {
			next.ServeHTTP(w, r)
			return
		}

		buf := &bufferedWriter{ResponseWriter: w}
		next.ServeHTTP(buf, r)
		if buf.status == 0 {
			buf.status = http.StatusOK
		}
		if buf.status != http.StatusOK {
			w.WriteHeader(buf.status)
			w.Write(buf.body.Bytes())
			return
		}

		h := fnv.New64a()
		h.Write(buf.body.Bytes())
		tag := fmt.Sprintf(`W/"%016x"`, h.Sum64())
		w.Header().Set("ETag", tag)
		w.Header().Set("Cache-Control", "private, no-cache")
		if etagMatches(r.Header.Get("If-None-Match"), tag) {
			w.Header().Del("Content-Type")
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(buf.body.Bytes())
	})
}

// etagMatches compara do jeito fraco (RFC 9110, 13.1.2): W/"x" e "x" são a mesma etiqueta
func etagMatches(ifNoneMatch, tag string) bool {
	want := strings.TrimPrefix(tag, "W/")
	for _, t := range strings.Split(ifNoneMatch, ",") {
		t = strings.TrimSpace(t)
		if t == "*" || (t != "" && strings.TrimPrefix(t, "W/") == want) {
			return true
		}
	}
	return false
}
