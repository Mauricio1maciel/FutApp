package handlers

import (
	"App-Futebol/services"
	"App-Futebol/utils"
	"encoding/json"
	"net/http"

	"github.com/patrickmn/go-cache"
)

func LiveMatchesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	league := r.URL.Query().Get("league")
	date := r.URL.Query().Get("date")

	if league == "" {
		http.Error(w, "Informe a liga (ex: ?league=BSA)", http.StatusBadRequest)
		return
	}
	cacheKey := "live_" + league + "_" + date
	if cachedData, found := utils.AppCache.Get(cacheKey); found {
		w.Header().Set("X-Cache", "HIT")
		w.Write(cachedData.([]byte))
		return
	}
	matches, err := services.GetLiveScoreboard(league, date)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, `{"error": "Erro ao buscar partidas ao vivo"}`, http.StatusInternalServerError)
		return
	}
	jsonBytes, err := json.Marshal(matches)
	if err != nil {
		http.Error(w, `{"error": "Erro ao processar dados"}`, http.StatusInternalServerError)
		return
	}
	utils.AppCache.Set(cacheKey, jsonBytes, cache.DefaultExpiration)
	w.Header().Set("X-Cache", "MISS")
	w.Write(jsonBytes)
}
