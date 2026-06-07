package handlers

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"App-Futebol/utils"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

var lastReset time.Time
var requestCount int

func canUpdate(force bool) bool {

	now := time.Now()

	if now.Sub(lastReset) > time.Minute {
		requestCount = 0
		lastReset = now
	}

	limit := 3
	if force {
		limit = 8
	}

	if requestCount >= limit {
		return false
	}

	requestCount++
	return true
}

func getSeasonFromDate(dateStr string) string {

	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return strconv.Itoa(time.Now().Year())
	}

	year := t.Year()
	month := t.Month()

	if month >= time.January && month <= time.December {
	}

	if month >= time.July {
		return strconv.Itoa(year) + "-" + strconv.Itoa(year+1)
	}

	return strconv.Itoa(year-1) + "-" + strconv.Itoa(year)
}

func MatchesHandler(w http.ResponseWriter, r *http.Request) {

	league := r.URL.Query().Get("league")
	roundStr := r.URL.Query().Get("round")
	forceUpdate := r.URL.Query().Get("update") == "true"

	if league == "" {
		http.Error(w, "Informe a liga (ex: BSA, PL, PD)", http.StatusBadRequest)
		return
	}
	if roundStr == "" {
		currentRoundInt, _ := database.GetCurrentRound(league)
		roundStr = strconv.Itoa(currentRoundInt)
	}

	if canUpdate(forceUpdate) {

		utils.CustomLog("API", "Atualizando dados da API: %s", league)

		apiMatches, err := services.GetMatchesByLeagueCode(league)

		if err != nil {
			log.Println("❌ Erro ao buscar API:", err)
		} else {

			for _, m := range apiMatches {

				homeScore := 0
				awayScore := 0

				if m.Score.FullTime.Home != nil {
					homeScore = *m.Score.FullTime.Home
				}

				if m.Score.FullTime.Away != nil {
					awayScore = *m.Score.FullTime.Away
				}

				err := database.SaveMatch(
					int64(m.ID),
					league,
					getSeasonFromDate(m.UTCDate),
					m.Matchday,
					int64(m.HomeTeam.ID),
					int64(m.AwayTeam.ID),
					homeScore,
					awayScore,
					m.UTCDate,
					m.Status,
				)

				if err != nil {
					log.Println("❌ Erro ao salvar:", err)
				}
			}
		}
	} else {
		log.Println("⏱ Limite de requisições atingido")
	}

	matches, err := database.GetMatchesByLeague(league, roundStr)

	if err != nil {
		log.Printf("Erro ao buscar jogos no banco: %v", err)
		http.Error(w, "Erro ao buscar jogos", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matches)
}
