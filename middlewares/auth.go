// Arquivo: middlewares/auth.go
package middlewares

import (
	"App-Futebol/utils"
	"context"
	"fmt"
	"net/http"
	"strings"
)

type contextKey string

const claimsKey contextKey = "claims"

func JWTAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		claims, errMsg := claimsFromHeader(r)
		if claims == nil {
			utils.WriteError(w, http.StatusUnauthorized, errMsg)
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// claimsFromHeader lê e valida o "Authorization: Bearer <token>"
func claimsFromHeader(r *http.Request) (*utils.Claims, string) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, "Token não fornecido"
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, "Formato de token inválido"
	}

	claims, err := utils.ValidateToken(strings.TrimSpace(parts[1]))
	if err != nil {
		fmt.Printf("\n❌ ERRO DE VALIDAÇÃO DO TOKEN: %v\n", err)
		return nil, "Token inválido ou expirado"
	}

	return claims, ""
}

// IsAdmin indica se a requisição veio de um usuário admin logado.
// Usado pelos handlers para liberar update=true / force_update=true.
func IsAdmin(r *http.Request) bool {
	claims, _ := r.Context().Value(claimsKey).(*utils.Claims)
	return claims.IsAdmin()
}

// DeviceID devolve o device_id do token de convidado ("" para token de admin, que não tem)
func DeviceID(r *http.Request) string {
	claims, _ := r.Context().Value(claimsKey).(*utils.Claims)
	if claims == nil {
		return ""
	}
	return claims.DeviceID
}
