package handlers

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"log"
	"net/http"
)

func ForceSyncHistoryHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	if league == "" {
		league = "BSA"
	}
	missing, err := database.GetMissingMatches(league)
	if err != nil {
		log.Printf("Erro ao buscar jogos faltantes: %v", err)
		http.Error(w, "Erro interno ao buscar dados", 500)
		return
	}
	for _, m := range missing {
		log.Printf("🔄 Sincronizando: %s vs %s na data %s", m["home"], m["away"], m["date"])
		err := services.UpdateMatchFromESPN(m["home"], m["away"], m["date"], league)
		if err != nil {
			log.Printf("⚠️ Falha ao sincronizar %s: %v", m["home"], err)
			continue
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "concluido", "message": "Sincronização de histórico finalizada"}`))
}
