package handlers

import (
	"App-Futebol/database"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func TeamMatchesHandler(w http.ResponseWriter, r *http.Request) {
	teamIDStr := r.URL.Query().Get("id")
	roundsStr := r.URL.Query().Get("rounds")

	if teamIDStr == "" {
		http.Error(w, `{"error": "O parâmetro 'id' é obrigatório"}`, http.StatusBadRequest)
		return
	}
	teamID, err := strconv.Atoi(teamIDStr)
	if err != nil {
		http.Error(w, `{"error": "O 'id' deve ser um número válido"}`, http.StatusBadRequest)
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
				http.Error(w, `{"error": "O parâmetro 'rounds' deve conter apenas números separados por vírgula"}`, http.StatusBadRequest)
				return
			}
			rounds = append(rounds, round)
		}
	}

	matches, err := database.GetMatchesByTeamID(int64(teamID), rounds)
	if err != nil {
		http.Error(w, `{"error": "Erro ao buscar jogos do time"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matches)
}
