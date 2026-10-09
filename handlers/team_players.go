package handlers

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"App-Futebol/utils"
	"fmt"
	"net/http"
	"strconv"
)

func TeamPlayersHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	if league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Parâmetro league é obrigatório")
		return
	}

	teamIDStr := r.URL.Query().Get("teamID")
	if teamIDStr == "" {
		utils.WriteError(w, http.StatusBadRequest, "Parâmetro teamID é obrigatório")
		return
	}

	teamID, err := strconv.ParseInt(teamIDStr, 10, 64)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "teamID inválido")
		return
	}

	espnTeamID, err := database.GetESPNTeamID(r.Context(), teamID)
	if err != nil {
		espnTeamID = ""
	}

	if espnTeamID != "" && espnTeamID != "0" {
		espnTeamIDInt, err := strconv.ParseInt(espnTeamID, 10, 64)
		if err == nil {
			espnPlayers, err := database.GetESPNPlayersByTeamID(r.Context(), int(espnTeamIDInt))

			if err == nil && len(espnPlayers) > 0 {
				utils.WriteJSON(w, http.StatusOK, espnPlayers)
				return
			}
		}
	}

	// Sem elenco da ESPN: usa o elenco da football-data já salvo no banco
	fallbackPlayers, err := database.GetTeamPlayersBy(r.Context(), teamID, league)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar elenco")
		return
	}

	for i := range fallbackPlayers {
		fallbackPlayers[i].Source = "DATA"
	}

	utils.WriteJSON(w, http.StatusOK, fallbackPlayers)
}

func SyncESPNTeamHandler(w http.ResponseWriter, r *http.Request) {
	teamIDStr := r.URL.Query().Get("teamID")
	leagueCode := r.URL.Query().Get("league")

	if teamIDStr == "" || leagueCode == "" {
		utils.WriteError(w, http.StatusBadRequest, "Os parâmetros teamID e league são obrigatórios")
		return
	}

	teamID, _ := strconv.ParseInt(teamIDStr, 10, 64)

	espnTeamID, err := database.GetESPNTeamID(r.Context(), teamID)
	if err != nil || espnTeamID == "" || espnTeamID == "0" {
		utils.WriteError(w, http.StatusNotFound, "Este time não possui espn_team_id mapeado no banco")
		return
	}

	var espnLeagueSlug string
	query := `SELECT COALESCE(code_espn, '') FROM leagues WHERE code_api = $1`
	err = database.DB.QueryRow(query, leagueCode).Scan(&espnLeagueSlug)

	if err != nil || espnLeagueSlug == "" {
		utils.WriteError(w, http.StatusNotFound, "Esta liga não possui code_espn mapeado na tabela leagues")
		return
	}

	espnTeamIDInt, _ := strconv.ParseInt(espnTeamID, 10, 64)
	err = services.SyncESPNRoster(r.Context(), espnLeagueSlug, int(espnTeamIDInt))
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, fmt.Sprintf("Falha ao baixar os jogadores da ESPN: %v", err))
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "Elenco sincronizado com sucesso da ESPN!",
		"team_api_id":  teamID,
		"espn_team_id": espnTeamID,
		"espn_league":  espnLeagueSlug,
	})
}
