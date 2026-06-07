package handlers

import (
	"App-Futebol/database"
	"encoding/json"
	"log"
	"net/http"
)

func SyncTeamsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Disparando sincronização manual de times...")

	var espnTeams map[string]int64

	if r.Body != nil {
		err := json.NewDecoder(r.Body).Decode(&espnTeams)
		if err != nil {
			log.Printf("Aviso: Nenhum JSON válido no request. %v", err)
		}
	}

	if espnTeams == nil {
		espnTeams = make(map[string]int64)
		log.Println("Aviso: Nenhum dado de time recebido. Sincronização rodará com mapa vazio ou você pode plugar o scraper diretamente aqui.")
	}

	err := database.SyncCrossAPITeams(espnTeams)
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
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "sucesso",
		"message": "A sincronização dos IDs das equipes foi concluída!",
	})
}
