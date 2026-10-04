// Arquivo: middlewares/admin.go
package middlewares

import (
	"crypto/subtle"
	"net/http"
	"os"
)

// AdminAuth exige o header X-Admin-Key igual à variável de ambiente ADMIN_KEY.
// Sem ADMIN_KEY configurada, as rotas admin ficam bloqueadas.
func AdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminKey := os.Getenv("ADMIN_KEY")
		if adminKey == "" {
			http.Error(w, `{"erro": "Rotas admin desabilitadas"}`, http.StatusServiceUnavailable)
			return
		}

		providedKey := r.Header.Get("X-Admin-Key")
		if subtle.ConstantTimeCompare([]byte(providedKey), []byte(adminKey)) != 1 {
			http.Error(w, `{"erro": "Acesso negado"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	}
}
