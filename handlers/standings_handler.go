package handlers

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"App-Futebol/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func getSeasonByLeague(league string) string {
	now := time.Now()
	currentYear := now.Year()
	currentMonth := now.Month()

	switch league {
	case "BSA":
		return fmt.Sprintf("%d", currentYear)
	case "PD", "PL", "SA", "BL1", "FL1":
		if currentMonth >= time.July {
			return fmt.Sprintf("%d-%d", currentYear, currentYear+1)
		}
		return fmt.Sprintf("%d-%d", currentYear-1, currentYear)
	}
	return fmt.Sprintf("%d", currentYear)
}

var lastStandingsUpdate = make(map[string]time.Time)

func canUpdateStandingsBackground(league string) bool {
	now := time.Now()
	last, exists := lastStandingsUpdate[league]

	if !exists || now.Sub(last) > 15*time.Minute {
		lastStandingsUpdate[league] = now
		return true
	}
	return false
}

func StandingsHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	if league == "" {
		http.Error(w, "Informe a liga", http.StatusBadRequest)
		return
	}

	season := getSeasonByLeague(league)

	result, err := database.GetStandingsByLeague(league, season)

	if err == nil && len(result) > 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)

		go processStandingsInBackground(league, season)
		return
	}

	utils.CustomLog("API", "Tabela vazia. Calculando pela primeira vez: %s", league)
	forceCalculateAndSaveStandings(league, season)

	result, _ = database.GetStandingsByLeague(league, season)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func processStandingsInBackground(league, season string) {
	// A SUA IDEIA: Se o campeonato inteiro já acabou, não calcula NUNCA MAIS!
	if database.IsLeagueFinished(league) {
		utils.CustomLog("API", "Liga %s finalizada. Poupando CPU do Render.", league)
		return
	}

	if !canUpdateStandingsBackground(league) {
		return
	}

	utils.CustomLog("API", "Atualizando tabela no fundo (Goroutine): %s", league)
	forceCalculateAndSaveStandings(league, season)
}

func forceCalculateAndSaveStandings(league, season string) {
	matches, err := database.GetMatchesByLeague(league, "")
	if err != nil {
		return
	}

	winners, _ := database.GetWinnersBySeasonAndSeason(league, season)
	rule, _ := database.GetCompetitionRule(league, season)
	zones, _ := database.GetZonesByLeague(league)
	criteria, _ := database.GetTieBreakers(league, season)

	standings := services.BuildStandings(matches, winners, rule, zones, criteria)

	for i := range standings {
		standings[i].Season = season
	}

	database.ClearStandings(league, season)
	database.SaveStandings(league, season, standings)
}
