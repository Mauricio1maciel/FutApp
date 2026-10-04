package handlers

import (
	"App-Futebol/services"
	"App-Futebol/utils"
	"log"
	"net/http"
)

func SyncTeamsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Disparando sincronização manual de times...")

	linked, err := services.SyncESPNTeamLinks()
	if err != nil {
		log.Printf("[ERRO] Falha na sincronização manual: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Falha ao sincronizar times")
		return
	}

	log.Println("Sincronização manual concluída com sucesso!")
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Sincronização concluída. Veja os logs [SYNC_TEAMS] para os times não vinculados.",
		"linked":  linked,
	})
}
