package handlers

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/services"
	"App-Futebol/utils"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

func getSeasonByLeague(league string) string {
	t := time.Now()

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
	forceUpdate := r.URL.Query().Get("update") == "true"

	if league == "" {
		http.Error(w, "Informe a liga", http.StatusBadRequest)
		return
	}

	season := getSeasonByLeague(league)

	// Se NÃO forçou a atualização, tenta buscar do banco
	if !forceUpdate {
		result, err := database.GetStandingsByLeague(league, season)

		if err == nil && len(result) > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)

			go processStandingsInBackground(league, season)
			return
		}
	}

	// Se chegou aqui, ou o banco está vazio, ou forçamos com ?update=true
	utils.CustomLog("API", "Tabela vazia ou atualização forçada. Calculando: %s", league)
	forceCalculateAndSaveStandings(league, season)

	result, _ := database.GetStandingsByLeague(league, season)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func processStandingsInBackground(league, season string) {
	if database.IsLeagueFinished(league) {
		utils.CustomLog("API", "Liga %s finalizada. Poupando CPU.", league)
		return
	}

	if !canUpdateStandingsBackground(league) {
		return
	}

	utils.CustomLog("API", "Atualizando tabela no fundo (Goroutine): %s", league)
	forceCalculateAndSaveStandings(league, season)
}

func forceCalculateAndSaveStandings(league, season string) {
	matches, err := database.GetMatchesByLeague(league, "", "", false)
	if err != nil {
		return
	}

	winners, _ := database.GetWinnersBySeasonAndSeason(league, season)
	rule, _ := database.GetCompetitionRule(league, season)
	zones, _ := database.GetZonesByLeague(league)
	criteria, _ := database.GetTieBreakers(league, season)

	var standings []models.Standing

	// 🔥 O DESVIO INTELIGENTE
	if league == "WC" {
		standings = services.BuildCupStandings(matches, criteria)
	} else {
		standings = services.BuildStandings(matches, winners, rule, zones, criteria)
	}

	for i := range standings {
		standings[i].Season = season
	}

	database.ClearStandings(league, season)
	database.SaveStandings(league, season, standings)
}
