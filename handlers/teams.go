package handlers

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

func getSmartSeason(league string) string {
	now := time.Now()
	year := now.Year()
	month := now.Month()

	format := database.GetLeagueSeasonFormat(league)

	if format == "calendar" {
		return strconv.Itoa(year)
	}

	if month >= time.July {
		return strconv.Itoa(year) + "-" + strconv.Itoa(year+1)
	}
	return strconv.Itoa(year-1) + "-" + strconv.Itoa(year)
}

func TeamsHandler(w http.ResponseWriter, r *http.Request) {
	league := r.URL.Query().Get("league")
	season := r.URL.Query().Get("season")
	update := r.URL.Query().Get("update")

	if league == "" {
		http.Error(w, `{"error": "Informe a league"}`, http.StatusBadRequest)
		return
	}

	if season == "" {
		season = getSmartSeason(league)
	}
	if update != "true" {
		teams, err := database.GetTeamsByLeague(league)
		if err == nil && len(teams) > 0 {
			json.NewEncoder(w).Encode(teams)
			return
		}
	}

	teams, err := services.GetTeams(league)
	if err != nil {
		http.Error(w, `{"error": "Erro ao buscar times na API"}`, http.StatusInternalServerError)
		return
	}

	savedCount := 0
	for _, team := range teams {
		err := database.SaveTeam(
			int64(team.ID),
			team.Name,
			team.Short,
			team.TLA,
			league,
			team.Stadium,
			team.Crest,
			season)
		if err == nil {
			savedCount++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Times atualizados com sucesso!",
		"league":  league,
		"season":  season,
		"count":   savedCount,
	})
}
