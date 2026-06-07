// Arquivo: middlewares/auth.go
package middlewares

import (
	"App-Futebol/utils"
	"context"
	"fmt"
	"net/http"
	"strings"
)

func JWTAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"erro": "Token não fornecido"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, `{"erro": "Formato de token inválido"}`, http.StatusUnauthorized)
			return
		}

		tokenString := parts[1] // Aqui está o token gigante

		// Remove possíveis espaços ou quebras de linha invisíveis que o Insomnia possa ter enviado
		tokenString = strings.TrimSpace(tokenString)

		claims, err := utils.ValidateToken(tokenString)
		if err != nil {
			// 🔥 A MÁGICA ESTÁ AQUI: Vai imprimir no seu terminal por que o token falhou!
			fmt.Printf("\n❌ ERRO DE VALIDAÇÃO DO TOKEN: %v\n", err)
			http.Error(w, `{"erro": "Token inválido ou expirado"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "device_id", claims.DeviceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
