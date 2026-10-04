package utils

import (
	"encoding/json"
	"net/http"
)

// WriteJSON responde com JSON e o Content-Type correto
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError é o único formato de erro da API: {"error": "mensagem"}
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}
