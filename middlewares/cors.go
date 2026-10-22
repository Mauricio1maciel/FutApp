// Arquivo: middlewares/cors.go
package middlewares

import (
	"net/http"
	"os"
	"strings"
)

// CORS libera a API para o site (a versão web do app), que roda em outro domínio.
//
// Só o navegador manda o header Origin: o app (React Native) não usa CORS e nada muda
// para ele. A autenticação é pelo header Authorization (Bearer), nunca por cookie, então
// liberar qualquer origem não expõe a sessão de ninguém: um site de terceiros não tem o
// token do usuário.
//
// CORS_ALLOWED_ORIGINS restringe as origens, separadas por vírgula (ex:
// "https://futstatis.vercel.app,http://localhost:3000"). Vazio ou "*": qualquer origem.
//
// O preflight (o OPTIONS que o navegador manda antes de uma chamada com Authorization)
// é respondido aqui, antes do JWTAuth e dos handlers: sem token ele levaria 401, e no
// /auth/login daria 405 e contaria como tentativa de login.
func CORS(next http.Handler) http.Handler {
	allowAll, allowed := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		h := w.Header()
		originAllowed := allowAll || allowed[normalizeOrigin(origin)]
		if !allowAll {
			// A resposta muda conforme a origem: proxies e o navegador não podem reaproveitá-la para outra
			h.Add("Vary", "Origin")
		}
		if originAllowed {
			if allowAll {
				h.Set("Access-Control-Allow-Origin", "*")
			} else {
				h.Set("Access-Control-Allow-Origin", origin)
			}
			// Deixa o site ler o X-Cache (HIT/MISS/BYPASS), como o app, e o Retry-After do 429
			h.Set("Access-Control-Expose-Headers", "X-Cache, Retry-After")
		}

		isPreflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
		if isPreflight {
			if originAllowed {
				h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				// O navegador guarda a liberação e não repete o preflight a cada chamada
				h.Set("Access-Control-Max-Age", "86400")
			}
			// Origem não liberada: 204 sem os headers, e o navegador bloqueia a chamada
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func normalizeOrigin(origin string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(origin), "/"))
}

func parseAllowedOrigins(value string) (allowAll bool, origins map[string]bool) {
	origins = map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		origin := normalizeOrigin(item)
		switch origin {
		case "":
		case "*":
			allowAll = true
		default:
			origins[origin] = true
		}
	}
	if len(origins) == 0 {
		allowAll = true
	}
	return allowAll, origins
}
