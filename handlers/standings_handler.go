package handlers

import (
	"App-Futebol/database"
	"App-Futebol/services"
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

func StandingsHandler(w http.ResponseWriter, r *http.Request) {

	league := r.URL.Query().Get("league")

	if league == "" {
		http.Error(w, "Informe a liga", http.StatusBadRequest)
		return
	}

	season := getSeasonByLeague(league)

	matches, err := database.GetMatchesByLeague(league, "")
	if err != nil {
		http.Error(w, "Erro ao buscar partidas", http.StatusInternalServerError)
		return
	}

	winners, err := database.GetWinnersBySeasonAndSeason(league, season)
	if err != nil {
		http.Error(w, "Erro ao buscar campeões", http.StatusInternalServerError)
		return
	}

	rule, err := database.GetCompetitionRule(league, season)
	if err != nil {
		http.Error(w, "Erro ao buscar regras da competição", http.StatusInternalServerError)
		return
	}

	zones, err := database.GetZonesByLeague(league)
	if err != nil {
		http.Error(w, "Erro ao buscar zonas da competição", http.StatusInternalServerError)
		return
	}
	criteria, err := database.GetTieBreakers(league, season)
	if err != nil {
		http.Error(w, "Erro ao buscar critérios de desempate", http.StatusInternalServerError)
		return
	}

	standings := services.BuildStandings(
		matches,
		winners,
		rule,
		zones,
		criteria,
	)

	for i := range standings {
		standings[i].Season = season
	}

	err = database.ClearStandings(league, season)
	if err != nil {
		http.Error(w, "Erro ao limpar standings", http.StatusInternalServerError)
		return
	}

	err = database.SaveStandings(league, season, standings)
	if err != nil {
		http.Error(w, "Erro ao salvar standings", http.StatusInternalServerError)
		return
	}

	result, err := database.GetStandingsByLeague(league, season)
	if err != nil {
		http.Error(w, "Erro ao buscar standings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
