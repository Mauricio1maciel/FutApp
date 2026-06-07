// Arquivo: handlers/auth.go
package handlers

import (
	"App-Futebol/utils"
	"encoding/json"
	"net/http"
)

type GuestLoginRequest struct {
	DeviceID string `json:"device_id"`
}

// GuestAuthHandler gera o token silencioso para o app
func GuestAuthHandler(w http.ResponseWriter, r *http.Request) {
	// Só aceita método POST
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req GuestLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DeviceID == "" {
		http.Error(w, "Device ID inválido ou ausente", http.StatusBadRequest)
		return
	}

	// Chama a função que criamos no utils para gerar o token
	tokenString, err := utils.GenerateToken(req.DeviceID)
	if err != nil {
		utils.CustomLog("AUTH_ERRO", "Falha ao gerar JWT: %v", err)
		http.Error(w, "Erro interno ao gerar token", http.StatusInternalServerError)
		return
	}

	utils.CustomLog("AUTH", "Novo dispositivo registrado/renovado: %s", req.DeviceID)

	// Devolve o token para o React Native salvar
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": tokenString,
		"type":  "Bearer",
	})
}
