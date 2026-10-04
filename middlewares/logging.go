// Arquivo: middlewares/logging.go
package middlewares

import (
	"App-Futebol/utils"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// statusRecorder guarda o status que o handler respondeu, para o log
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}

// RequestLogger registra método, rota, status e tempo de cada requisição.
// O /health só aparece no log quando falha (o Render chama a todo instante).
// O token nunca é logado: ele vem no header, não na URL.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		if r.URL.Path == "/health" && rec.status == http.StatusOK {
			return
		}
		utils.CustomLog("HTTP", "%s %s %d %s", r.Method, r.URL.RequestURI(), rec.status, time.Since(start).Round(time.Millisecond))
	})
}

// Recover transforma um panic num handler em 500 com JSON, com o stack trace no log,
// em vez de derrubar a conexão sem resposta
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				if err == http.ErrAbortHandler {
					panic(err) // cliente desconectou: comportamento padrão do net/http
				}
				log.Printf("[PANIC] %s %s: %v\n%s", r.Method, r.URL.Path, err, debug.Stack())
				utils.WriteError(w, http.StatusInternalServerError, "Erro interno")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
