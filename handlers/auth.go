// Arquivo: handlers/auth.go
package handlers

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type GuestLoginRequest struct {
	DeviceID string `json:"device_id"`
}

func GuestAuthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Método não permitido")
		return
	}

	var req GuestLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DeviceID == "" {
		utils.WriteError(w, http.StatusBadRequest, "Device ID inválido ou ausente")
		return
	}

	tokenString, err := utils.GenerateToken(req.DeviceID)
	if err != nil {
		utils.CustomLog("AUTH_ERRO", "Falha ao gerar JWT: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Erro interno ao gerar token")
		return
	}

	utils.CustomLog("AUTH", "Novo dispositivo registrado/renovado: %s", req.DeviceID)
	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"token": tokenString,
		"type":  "Bearer",
	})
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Hash usado quando o e-mail não existe, para a resposta demorar o mesmo tempo
// e não revelar quais e-mails têm conta
var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("senha-inexistente"), bcrypt.DefaultCost)

// Proteção contra força bruta: 5 erros seguidos bloqueiam o IP por 15 minutos
const (
	maxLoginFailures = 5
	loginBlockWindow = 15 * time.Minute
)

type loginAttempt struct {
	failures int
	first    time.Time
}

var (
	loginAttempts   = make(map[string]*loginAttempt)
	loginAttemptsMu sync.Mutex
)

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func loginBlocked(ip string) bool {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()

	a, ok := loginAttempts[ip]
	if !ok {
		return false
	}
	if time.Since(a.first) > loginBlockWindow {
		delete(loginAttempts, ip)
		return false
	}
	return a.failures >= maxLoginFailures
}

func registerLoginFailure(ip string) {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()

	a, ok := loginAttempts[ip]
	if !ok || time.Since(a.first) > loginBlockWindow {
		loginAttempts[ip] = &loginAttempt{failures: 1, first: time.Now()}
		return
	}
	a.failures++
}

func clearLoginFailures(ip string) {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()
	delete(loginAttempts, ip)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Método não permitido")
		return
	}

	ip := clientIP(r)
	if loginBlocked(ip) {
		utils.CustomLog("AUTH", "Login bloqueado por excesso de tentativas: %s", ip)
		utils.WriteError(w, http.StatusTooManyRequests, "Muitas tentativas. Tente novamente em alguns minutos.")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe email e password")
		return
	}

	user, err := database.GetUserByEmail(r.Context(), req.Email)
	hash := []byte(user.PasswordHash)
	if err != nil {
		hash = dummyPasswordHash
	}

	if bcrypt.CompareHashAndPassword(hash, []byte(req.Password)) != nil || err != nil {
		registerLoginFailure(ip)
		utils.CustomLog("AUTH", "Falha de login para %s (IP %s)", req.Email, ip)
		utils.WriteError(w, http.StatusUnauthorized, "E-mail ou senha inválidos")
		return
	}

	clearLoginFailures(ip)

	tokenString, err := utils.GenerateUserToken(user.ID, user.Role)
	if err != nil {
		utils.CustomLog("AUTH", "Falha ao gerar JWT: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Erro interno ao gerar token")
		return
	}

	database.TouchUserLogin(r.Context(), user.ID)
	utils.CustomLog("AUTH", "Login de %s (%s)", user.Email, user.Role)

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"token": tokenString,
		"type":  "Bearer",
		"role":  user.Role,
	})
}
