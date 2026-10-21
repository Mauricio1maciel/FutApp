package handlers

import (
	"App-Futebol/services"
	"App-Futebol/utils"
	"net/http"
	"strconv"
)

// PlayerStatsHandler: GET /player/stats?id=<ID da ESPN>[&season=2026]
// Jogos, minutos, gols e cartões por competição, e os últimos 10 jogos do jogador.
func PlayerStatsHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "Informe o id do jogador (o mesmo do /details?type=player)")
		return
	}
	stats, err := services.GetPlayerStats(r.Context(), id, r.URL.Query().Get("season"))
	if err != nil {
		utils.CustomLog("DB_ERRO", "Erro nas estatísticas do jogador %d: %v", id, err)
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao calcular as estatísticas do jogador")
		return
	}
	utils.WriteJSON(w, http.StatusOK, stats)
}

// TeamStatsHandler: GET /team/stats?id=<api_id>[&season=2026]
// Campanha, médias de jogo e artilheiros do time em cada competição.
func TeamStatsHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "Informe o id do time (api_id)")
		return
	}
	stats, err := services.GetTeamStats(r.Context(), id, r.URL.Query().Get("season"))
	if err != nil {
		utils.CustomLog("DB_ERRO", "Erro nas estatísticas do time %d: %v", id, err)
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao calcular as estatísticas do time")
		return
	}
	utils.WriteJSON(w, http.StatusOK, stats)
}
