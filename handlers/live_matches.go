package handlers

import (
	"App-Futebol/models"
	"App-Futebol/services"
	"App-Futebol/utils"
	"encoding/json"
	"net/http"

	"github.com/patrickmn/go-cache"
)

func LiveMatchesHandler(w http.ResponseWriter, r *http.Request) {

	league := r.URL.Query().Get("league")
	date := r.URL.Query().Get("date")

	if league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe a liga (ex: ?league=BSA)")
		return
	}
	cacheKey := "live_" + league + "_" + date
	if cachedData, found := utils.AppCache.Get(cacheKey); found {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Cache", "HIT")
		w.Write(cachedData.([]byte))
		return
	}
	matches, err := services.GetLiveScoreboard(league, date)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, "Erro ao buscar partidas ao vivo")
		return
	}
	if matches == nil {
		matches = []models.AppLiveMatch{}
	}
	jsonBytes, err := json.Marshal(matches)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao processar dados")
		return
	}
	utils.AppCache.Set(cacheKey, jsonBytes, cache.DefaultExpiration)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Cache", "MISS")
	w.Write(jsonBytes)
}
