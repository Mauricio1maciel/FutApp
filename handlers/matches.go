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
		if err := services.SyncFootballDataMatches(league); err != nil {
			log.Printf("Erro ao atualizar jogos na football-data: %v", err)
		}
	}

	season = services.ResolveSeason(league, season)

	isCurrentRound := false

	if roundStr == "" {
		phase := database.GetCurrentPhase(league, season)

		if phase == "CURRENT_ROUND" {
			currentRoundInt, _ := database.GetCurrentRound(league, season)
			if currentRoundInt > 38 {
				currentRoundInt = 1
			}
			roundStr = strconv.Itoa(currentRoundInt)
		} else {
			roundStr = phase
		}
		isCurrentRound = true
	}

	matches, err := database.GetMatchesByLeague(league, roundStr, dateStr, season, isCurrentRound)
	if err != nil {
		log.Printf("Erro ao buscar jogos no banco: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar jogos")
		return
	}
	utils.WriteJSON(w, http.StatusOK, matches)
}
