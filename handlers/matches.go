package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/services"
	"App-Futebol/utils"
	"log"
	"net/http"
	"strconv"
)

func MatchesHandler(w http.ResponseWriter, r *http.Request) {

	league := r.URL.Query().Get("league")
	roundStr := r.URL.Query().Get("round")
	dateStr := r.URL.Query().Get("date")
	season := r.URL.Query().Get("season")
	// Só o admin pode forçar a busca na football-data; para os demais, o worker mantém o banco atualizado
	forceUpdate := r.URL.Query().Get("update") == "true" && middlewares.IsAdmin(r)

	if league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe a liga (ex: BSA, PL, PD)")
		return
	}
	if forceUpdate {
		utils.CustomLog("API", "Admin forçou atualização dos jogos: %s", league)
		if err := services.SyncFootballDataMatches(r.Context(), league); err != nil {
			log.Printf("Erro ao atualizar jogos na football-data: %v", err)
		}
	}

	season = services.ResolveSeason(r.Context(), league, season)

	filter := database.MatchFilter{Date: dateStr}
	switch {
	case dateStr != "":
		// Só a data
	case roundStr != "":
		// O app escolheu: número = rodada, texto = fase (ex: SEMI_FINALS)
		if n, err := strconv.Atoi(roundStr); err == nil {
			filter.Round = &n
		} else {
			filter.Stage = roundStr
		}
	default:
		// Sem round: a fase e a rodada atuais, do mesmo jeito para ligas e copas.
		// Mata-mata (rodada 0) traz a fase inteira; liga e fase de grupos, a rodada.
		// Sem fase gravada (temporadas antigas), filtra sempre pela rodada, para
		// nunca devolver a temporada inteira.
		stage, round, err := database.GetCurrentStage(r.Context(), league, season)
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar jogos")
			return
		}
		filter.Stage = stage
		if round > 0 || stage == "" {
			filter.Round = &round
		}
	}

	matches, err := database.GetMatchesByLeague(r.Context(), league, season, filter)
	if err != nil {
		log.Printf("Erro ao buscar jogos no banco: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar jogos")
		return
	}
	utils.WriteJSON(w, http.StatusOK, matches)
}
