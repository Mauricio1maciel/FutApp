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

// DeviceID devolve o aparelho do token ("" se o token não tem: admin que entrou sem device_id)
func DeviceID(r *http.Request) string {
	claims, _ := r.Context().Value(claimsKey).(*utils.Claims)
	if claims == nil {
		return ""
	}
	return claims.DeviceID
}

// UserID devolve a conta do token (0 para convidado)
func UserID(r *http.Request) int64 {
	claims, _ := r.Context().Value(claimsKey).(*utils.Claims)
	if claims == nil {
		return 0
	}
	return claims.UserID
}

// DeviceFromHeader devolve o aparelho do token do Authorization, nas rotas sem JWTAuth
// ("" sem token ou com token inválido)
func DeviceFromHeader(r *http.Request) string {
	claims, _ := claimsFromHeader(r)
	if claims == nil {
		return ""
	}
	return claims.DeviceID
}
