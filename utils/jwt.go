// Arquivo: utils/jwt.go
package utils

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Pega a chave secreta e GARANTE que ela seja do tipo []byte
func getSecretKey() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return []byte("chave_secreta_padrao_dev")
	}
	// O segredo está aqui: converter explicitamente
	return []byte(secret)
}

// Claims define o que vai "escrito" dentro do token
type Claims struct {
	DeviceID string `json:"device_id"`
	jwt.RegisteredClaims
}

// GenerateToken cria um novo token válido por 30 dias
func GenerateToken(deviceID string) (string, error) {
	expirationTime := time.Now().Add(30 * 24 * time.Hour)

	claims := &Claims{
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getSecretKey())
}

// ValidateToken lê e valida
func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Proteção extra: Garante que o método de assinatura usado foi realmente o HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de assinatura inesperado: %v", token.Header["alg"])
		}

		// Retorna a chave já convertida em []byte
		return getSecretKey(), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("token inválido")
	}

	return claims, nil
}
