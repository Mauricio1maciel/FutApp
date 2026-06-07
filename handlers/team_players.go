package handlers

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

func TeamPlayersHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	if league == "" {
		http.Error(w, `{"error": "Parâmetro league é obrigatório"}`, http.StatusBadRequest)
		return
	}

	teamIDStr := r.URL.Query().Get("teamID")
	if teamIDStr == "" {
		http.Error(w, `{"error": "Parâmetro teamID é obrigatório"}`, http.StatusBadRequest)
		return
	}

	teamID, err := strconv.ParseInt(teamIDStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "teamID inválido"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	espnTeamID, err := database.GetESPNTeamID(teamID)
	if err != nil {
		espnTeamID = ""
	}

	if espnTeamID != "" && espnTeamID != "0" {
		espnTeamIDInt, err := strconv.ParseInt(espnTeamID, 10, 64)
		if err == nil {
			espnPlayers, err := database.GetESPNPlayersByTeamID(int(espnTeamIDInt))

			if err == nil && len(espnPlayers) > 0 {
				json.NewEncoder(w).Encode(espnPlayers)
				return
			}
		}
	}

	fallbackPlayers, err := services.GetTeamPlayersBy(teamID, league)
	if err != nil {
		http.Error(w, `{"error": "Erro ao buscar elenco nas APIs"}`, http.StatusInternalServerError)
		return
	}

	for i := range fallbackPlayers {
		fallbackPlayers[i].Source = "DATA"
	}

	json.NewEncoder(w).Encode(fallbackPlayers)
}

func SyncESPNTeamHandler(w http.ResponseWriter, r *http.Request) {
	teamIDStr := r.URL.Query().Get("teamID")
	leagueCode := r.URL.Query().Get("league")

	if teamIDStr == "" || leagueCode == "" {
		http.Error(w, `{"error": "Os parâmetros teamID e league são obrigatórios"}`, http.StatusBadRequest)
		return
	}

	teamID, _ := strconv.ParseInt(teamIDStr, 10, 64)

	espnTeamID, err := database.GetESPNTeamID(teamID)
	if err != nil || espnTeamID == "" || espnTeamID == "0" {
		http.Error(w, `{"error": "Este time não possui espn_team_id mapeado no banco"}`, http.StatusNotFound)
		return
	}

	var espnLeagueSlug string
	query := `SELECT COALESCE(code_espn, '') FROM leagues WHERE code_api = $1`
	err = database.DB.QueryRow(query, leagueCode).Scan(&espnLeagueSlug)

	if err != nil || espnLeagueSlug == "" {
		http.Error(w, `{"error": "Esta liga não possui code_espn mapeado na tabela leagues"}`, http.StatusNotFound)
		return
	}

	espnTeamIDInt, _ := strconv.ParseInt(espnTeamID, 10, 64)
	err = services.SyncESPNRoster(espnLeagueSlug, int(espnTeamIDInt))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("Falha ao baixar os jogadores da ESPN: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "Elenco sincronizado com sucesso da ESPN!",
		"team_api_id":  teamID,
		"espn_team_id": espnTeamID,
		"espn_league":  espnLeagueSlug,
	})
}
