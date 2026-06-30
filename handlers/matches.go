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

func getSeasonFromDate(dateStr string, league string) string {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		t = time.Now()
	}

	year := t.Year()
	month := t.Month()

	format := database.GetLeagueSeasonFormat(league)

	if format == "calendar" {
		return strconv.Itoa(year)
	}

	if month >= time.July {
		return strconv.Itoa(year) + "-" + strconv.Itoa(year+1)
	}
	return strconv.Itoa(year-1) + "-" + strconv.Itoa(year)
}

func MatchesHandler(w http.ResponseWriter, r *http.Request) {

	league := r.URL.Query().Get("league")
	roundStr := r.URL.Query().Get("round")
	dateStr := r.URL.Query().Get("date")
	season := r.URL.Query().Get("season")
	forceUpdate := r.URL.Query().Get("update") == "true"

	if league == "" {
		http.Error(w, "Informe a liga (ex: BSA, PL, PD)", http.StatusBadRequest)
		return
	}
	if season == "" {
		season = database.GetLatestSeason(league)
	}

	isCurrentRound := false

	if roundStr == "" {
		phase := database.GetCurrentPhase(league, season)

		if phase == "CURRENT_ROUND" {
			// Liga ainda está em fase de grupos (numérica)
			currentRoundInt, _ := database.GetCurrentRound(league, season)
			if currentRoundInt > 38 {
				currentRoundInt = 1
			}
			roundStr = strconv.Itoa(currentRoundInt)
		} else {
			// Liga entrou em mata-mata!
			roundStr = phase
		}
		isCurrentRound = true
	}

	if canUpdate(forceUpdate) {
		utils.CustomLog("API", "Atualizando dados da API em segundo plano: %s", league)

		go func(lg string) {
			apiMatches, err := services.GetMatchesByLeagueCode(lg)
			if err != nil {
				log.Println("❌ Erro ao buscar API:", err)
				return
			}

			for _, m := range apiMatches {
				homeScore, awayScore := 0, 0
				if m.Score.FullTime.Home != nil {
					homeScore = *m.Score.FullTime.Home
				}
				if m.Score.FullTime.Away != nil {
					awayScore = *m.Score.FullTime.Away
				}

				database.SaveMatch(
					int64(m.ID), lg,
					getSeasonFromDate(m.UTCDate, lg),
					m.Matchday,
					int64(m.HomeTeam.ID), int64(m.AwayTeam.ID),
					homeScore, awayScore, m.UTCDate, m.Status,
					m.Stage, m.Group,
				)
			}
			utils.CustomLog("API", "Atualização em segundo plano concluída para: %s", lg)
		}(league)
	} else {
		log.Println("⏱ Limite de requisições atingido")
	}

	matches, err := database.GetMatchesByLeague(league, roundStr, dateStr, season, isCurrentRound)
	if err != nil {
		log.Printf("Erro ao buscar jogos no banco: %v", err)
		http.Error(w, "Erro ao buscar jogos", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matches)
}
