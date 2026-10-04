// Arquivo: middlewares/admin.go
package middlewares

import (
	"App-Futebol/utils"
	"context"
	"crypto/subtle"
	"net/http"
	"os"
)

// AdminAuth libera a rota para:
//   - um usuário admin logado (Authorization: Bearer <token do /auth/login>), ou
//   - quem enviar o header X-Admin-Key igual à variável ADMIN_KEY (útil para curl/scripts).
func AdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if claims, _ := claimsFromHeader(r); claims.IsAdmin() {
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		adminKey := os.Getenv("ADMIN_KEY")
		providedKey := r.Header.Get("X-Admin-Key")
		if adminKey != "" && subtle.ConstantTimeCompare([]byte(providedKey), []byte(adminKey)) == 1 {
			next.ServeHTTP(w, r)
			return
		}

		utils.WriteError(w, http.StatusForbidden, "Acesso restrito ao administrador")
	}
}
