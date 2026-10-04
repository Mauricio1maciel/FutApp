// Arquivo: utils/jwt.go
package utils

import (
	"App-Futebol/models"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Pega a chave secreta e GARANTE que ela seja do tipo []byte
// (o main.go impede a API de subir sem JWT_SECRET)
func getSecretKey() []byte {
	return []byte(os.Getenv("JWT_SECRET"))
}

// Claims define o que vai "escrito" dentro do token
type Claims struct {
	DeviceID string `json:"device_id,omitempty"`
	UserID   int64  `json:"user_id,omitempty"`
	Role     string `json:"role,omitempty"` // vazio em tokens antigos = convidado
	jwt.RegisteredClaims
}

func (c *Claims) IsAdmin() bool {
	return c != nil && c.Role == models.RoleAdmin
}

// GenerateToken cria um token de convidado (por aparelho) válido por 30 dias
func GenerateToken(deviceID string) (string, error) {
	return signClaims(&Claims{DeviceID: deviceID, Role: models.RoleGuest}, 30*24*time.Hour)
}

// GenerateUserToken cria o token de quem fez login. Dura menos que o de convidado
// porque pode carregar o papel de admin.
func GenerateUserToken(userID int64, role string) (string, error) {
	return signClaims(&Claims{UserID: userID, Role: role}, 7*24*time.Hour)
}

func signClaims(claims *Claims, duration time.Duration) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
		IssuedAt:  jwt.NewNumericDate(now),
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
