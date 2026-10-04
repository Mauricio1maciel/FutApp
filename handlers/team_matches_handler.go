package handlers

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"net/http"
	"strconv"
	"strings"
)

func TeamMatchesHandler(w http.ResponseWriter, r *http.Request) {
	teamIDStr := r.URL.Query().Get("id")
	roundsStr := r.URL.Query().Get("rounds")

	if teamIDStr == "" {
		utils.WriteError(w, http.StatusBadRequest, "O parâmetro 'id' é obrigatório")
		return
	}
	teamID, err := strconv.Atoi(teamIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "O 'id' deve ser um número válido")
		return
	}

	var rounds []int64
	if roundsStr == "" {
		currentRoundInt, _ := database.GetCurrentRoundTeam(teamIDStr)

		rounds = append(rounds, 0)

		for i := 0; i < 8; i++ {
			rounds = append(rounds, int64(currentRoundInt+i))
		}
	} else {
		for _, r := range strings.Split(roundsStr, ",") {
			round, err := strconv.ParseInt(strings.TrimSpace(r), 10, 64)
			if err != nil {
				utils.WriteError(w, http.StatusBadRequest, "O parâmetro 'rounds' deve conter apenas números separados por vírgula")
				return
			}
			rounds = append(rounds, round)
		}
	}

	matches, err := database.GetMatchesByTeamID(int64(teamID), rounds)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar jogos do time")
		return
	}
	utils.WriteJSON(w, http.StatusOK, matches)
}
