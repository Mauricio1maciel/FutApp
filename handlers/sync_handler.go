package handlers

import (
	"App-Futebol/services"
	"encoding/json"
	"log"
	"net/http"
)

func SyncTeamsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Disparando sincronização manual de times...")

	linked, err := services.SyncESPNTeamLinks()
	if err != nil {
		log.Printf("[ERRO] Falha na sincronização manual: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Falha ao sincronizar times",
		})
		return
	}

	log.Println("Sincronização manual concluída com sucesso!")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "sucesso",
		"message":  "A sincronização dos IDs das equipes foi concluída!",
		"linked":   linked,
		"detalhes": "Veja os logs [SYNC_TEAMS] para os times não vinculados",
	})
}
