package middlewares

import (
	"bytes"
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"
)

// Cache curto das respostas de leitura.
//
// O Render fica nos EUA e o banco em São Paulo: cada consulta custa ~185ms de ida e
// volta, e um pedido de lista faz duas ou três. Os dados só mudam quando o worker grava,
// e a mesma lista é pedida por todos os aparelhos (a tela inicial pede ~16 listas a
// cada ~12s). Servida da memória, a resposta não sai do servidor.
//
// Só GET com status 200 entra no cache. A chave é a rota com os parâmetros em ordem
// alfabética, igual para todos: as rotas cacheadas não dependem de quem pede. O admin
// sempre recebe dados frescos (e é quem usa update=true / force_update=true).

var responseCache = cache.New(time.Minute, 5*time.Minute)

type cachedResponse struct {
	body        []byte
	contentType string
}

// bodyRecorder guarda o que o handler escreveu, para poder colocar no cache
type bodyRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *bodyRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *bodyRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// CacheFor guarda a resposta da rota por ttl. Deve ficar dentro do JWTAuth, que é quem
// diz se o pedido é de admin.
func CacheFor(ttl time.Duration, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || IsAdmin(r) {
			w.Header().Set("X-Cache", "BYPASS")
			next(w, r)
			return
		}

		key := r.URL.Path + "?" + r.URL.Query().Encode()
		if v, found := responseCache.Get(key); found {
			c := v.(cachedResponse)
			w.Header().Set("Content-Type", c.contentType)
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			w.Write(c.body)
			return
		}

		w.Header().Set("X-Cache", "MISS")
		rec := &bodyRecorder{ResponseWriter: w}
		next(rec, r)
		if rec.status == http.StatusOK {
			responseCache.Set(key, cachedResponse{
				body:        rec.body.Bytes(),
				contentType: w.Header().Get("Content-Type"),
			}, ttl)
		}
	}
}
