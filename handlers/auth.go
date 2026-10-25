// Arquivo: handlers/auth.go
package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/models"
	"App-Futebol/utils"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
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
	// Aparelho de onde entrou: o que ele seguia como convidado passa para a conta, e ele
	// passa a receber os avisos dela
	DeviceID string `json:"device_id"`
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

// clientIP: o endereço real do cliente (Cloudflare primeiro; o X-Forwarded-For sozinho
// podia ser inventado pelo cliente para driblar o bloqueio de tentativas de login)
func clientIP(r *http.Request) string { return middlewares.ClientIP(r) }

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
	if !deviceOwned(w, r, req.DeviceID) {
		return
	}
	database.TouchUserLogin(r.Context(), user.ID)
	utils.CustomLog("AUTH", "Login de %s (%s)", user.Email, user.Role)
	respondWithAccount(w, r, user, req.DeviceID, http.StatusOK)
}

// deviceOwned: só liga à conta o aparelho cujo token de convidado veio no Authorization.
// Sem isso, quem soubesse o device_id de outra pessoa puxaria o aparelho dela para a sua conta
func deviceOwned(w http.ResponseWriter, r *http.Request, deviceID string) bool {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID != "" && middlewares.DeviceFromHeader(r) != deviceID {
		utils.WriteError(w, http.StatusForbidden, "Envie o token de convidado deste aparelho no Authorization")
		return false
	}
	return true
}

// respondWithAccount liga o aparelho à conta (levando o que ele seguia) e devolve o token
func respondWithAccount(w http.ResponseWriter, r *http.Request, user models.User, deviceID string, status int) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID != "" {
		if err := database.AttachDeviceToUser(r.Context(), deviceID, user.ID); err != nil {
			utils.CustomLog("AUTH_ERRO", "Falha ao ligar o aparelho %s à conta %d: %v", deviceID, user.ID, err)
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao ligar o aparelho à conta")
			return
		}
	}
	tokenString, err := utils.GenerateUserToken(user.ID, user.Role, deviceID)
	if err != nil {
		utils.CustomLog("AUTH_ERRO", "Falha ao gerar JWT: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Erro interno ao gerar token")
		return
	}
	utils.WriteJSON(w, status, map[string]interface{}{
		"token": tokenString,
		"type":  "Bearer",
		"role":  user.Role,
		"user":  accountJSON(user),
	})
}

func accountJSON(u models.User) map[string]interface{} {
	return map[string]interface{}{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "role": u.Role}
}

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	DeviceID    string `json:"device_id"`
}

// RegisterHandler cria a conta: POST /auth/register {"email", "password", "display_name"?,
// "device_id"?}. Com device_id, o que o aparelho seguia como convidado passa para a conta
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Método não permitido")
		return
	}
	var req RegisterRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if !deviceOwned(w, r, req.DeviceID) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(email); err != nil || len(email) > 254 || !strings.Contains(email, ".") {
		utils.WriteError(w, http.StatusBadRequest, "E-mail inválido")
		return
	}
	if len(req.Password) < 8 || len(req.Password) > 72 {
		utils.WriteError(w, http.StatusBadRequest, "A senha precisa ter de 8 a 72 caracteres")
		return
	}
	if len(req.DisplayName) > 60 {
		utils.WriteError(w, http.StatusBadRequest, "O nome pode ter até 60 caracteres")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro interno")
		return
	}
	user, err := database.CreateUser(r.Context(), email, string(hash), req.DisplayName)
	if errors.Is(err, database.ErrEmailTaken) {
		utils.WriteError(w, http.StatusConflict, "Já existe uma conta com este e-mail")
		return
	}
	if err != nil {
		utils.CustomLog("AUTH_ERRO", "Falha ao criar conta: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao criar a conta")
		return
	}
	utils.CustomLog("AUTH", "Conta criada: %s", user.Email)
	respondWithAccount(w, r, user, req.DeviceID, http.StatusCreated)
}

// RefreshHandler renova o token da conta (POST /auth/refresh, com o token ainda válido):
// quem usa o app todo dia não precisa entrar de novo
func RefreshHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Método não permitido")
		return
	}
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	tokenString, err := utils.GenerateUserToken(user.ID, user.Role, middlewares.DeviceID(r))
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro interno ao gerar token")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{"token": tokenString, "type": "Bearer", "role": user.Role, "user": accountJSON(user)})
}

// LogoutHandler: POST /auth/logout. O aparelho sai da conta e para de receber os avisos
// dela; o app volta a usar o token de convidado (que começa sem nada seguido)
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Método não permitido")
		return
	}
	if device := middlewares.DeviceID(r); device != "" && middlewares.UserID(r) != 0 {
		if err := database.DetachDevice(r.Context(), device); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao sair da conta")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// MeHandler: GET /me devolve a conta; DELETE /me {"password"} apaga a conta e tudo o que ela
// segue (exigência das lojas: quem cria conta precisa poder apagá-la pelo app)
func MeHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		out := accountJSON(user)
		out["created_at"] = user.CreatedAt
		utils.WriteJSON(w, http.StatusOK, out)
	case http.MethodDelete:
		var req struct {
			Password string `json:"password"`
		}
		json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req)
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
			utils.WriteError(w, http.StatusUnauthorized, "Senha incorreta")
			return
		}
		if err := database.DeleteUser(r.Context(), user.ID); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao apagar a conta")
			return
		}
		utils.CustomLog("AUTH", "Conta apagada: %s", user.Email)
		w.WriteHeader(http.StatusNoContent)
	default:
		utils.WriteError(w, http.StatusMethodNotAllowed, "Use GET ou DELETE")
	}
}

// currentUser: a conta do token. Convidado (sem conta) recebe 401
func currentUser(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	id := middlewares.UserID(r)
	if id == 0 {
		utils.WriteError(w, http.StatusUnauthorized, "Entre na sua conta")
		return models.User{}, false
	}
	user, err := database.GetUserByID(r.Context(), id)
	if errors.Is(err, database.ErrNotFound) {
		utils.WriteError(w, http.StatusUnauthorized, "Conta não encontrada: entre de novo")
		return models.User{}, false
	}
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar a conta")
		return models.User{}, false
	}
	return user, true
}
