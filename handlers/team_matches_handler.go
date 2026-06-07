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
	if roundsStr == "" {
		currentRoundInt, _ := database.GetCurrentRoundTeam(teamIDStr)
		var roundsArray []string
		for i := 0; i < 8; i++ {
			roundsArray = append(roundsArray, strconv.Itoa(currentRoundInt+i))
		}
		roundsStr = strings.Join(roundsArray, ",")
	}

	teamID, err := strconv.Atoi(teamIDStr)
	if err != nil {
		http.Error(w, `{"error": "O 'id' deve ser um número válido"}`, http.StatusBadRequest)
		return
	}

	matches, err := database.GetMatchesByTeamID(int64(teamID), roundsStr)
	if err != nil {
		http.Error(w, `{"error": "Erro ao buscar jogos do time"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matches)
}
