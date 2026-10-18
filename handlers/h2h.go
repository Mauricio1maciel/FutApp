package handlers

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/services"
	"App-Futebol/utils"
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
)

type h2hSummary struct {
	Played     int `json:"played"`
	Team1Wins  int `json:"team1_wins"`
	Draws      int `json:"draws"`
	Team2Wins  int `json:"team2_wins"`
	Team1Goals int `json:"team1_goals"`
	Team2Goals int `json:"team2_goals"`
}

// HeadToHeadHandler devolve o confronto direto entre dois times, em todas as competições
// e temporadas do banco: GET /h2h?team1=1783&team2=1769[&limit=10]
func HeadToHeadHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	team1, err1 := strconv.ParseInt(q.Get("team1"), 10, 64)
	team2, err2 := strconv.ParseInt(q.Get("team2"), 10, 64)
	if err1 != nil || err2 != nil || team1 <= 0 || team2 <= 0 || team1 == team2 {
		utils.WriteError(w, http.StatusBadRequest, "Informe dois times diferentes: ?team1=ID&team2=ID")
		return
	}
	limit := 10
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 50 {
		limit = l
	}

	matches, err := database.GetHeadToHead(r.Context(), team1, team2)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar o confronto direto")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"team1":   team1,
		"team2":   team2,
		"summary": summarizeHeadToHead(team1, matches),
		"matches": matches[:min(limit, len(matches))],
	})
}

// summarizeHeadToHead conta vitórias, empates e gols de todos os jogos (não só os da
// lista). Decisão por pênaltis conta como empate: o jogo terminou empatado.
func summarizeHeadToHead(team1 int64, matches []models.Match) h2hSummary {
	var s h2hSummary
	for _, m := range matches {
		g1, g2 := m.HomeScore, m.AwayScore
		if m.APIHomeTeamID != team1 {
			g1, g2 = g2, g1
		}
		s.Played++
		s.Team1Goals += g1
		s.Team2Goals += g2
		switch {
		case g1 > g2:
			s.Team1Wins++
		case g1 < g2:
			s.Team2Wins++
		default:
			s.Draws++
		}
	}
	return s
}

var historyRunning atomic.Bool

// AdminSyncSeasonHandler carrega uma temporada antiga da football-data (times, jogos e
// classificação) em background: POST /admin/sync-season?league=BSA&season=2024
func AdminSyncSeasonHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Use POST")
		return
	}
	league := r.URL.Query().Get("league")
	year, err := strconv.Atoi(r.URL.Query().Get("season"))
	if league == "" || err != nil || year < 2000 || year > 2100 {
		utils.WriteError(w, http.StatusBadRequest, "Informe league e season (ano em que a temporada começou, ex: 2024)")
		return
	}
	if !historyRunning.CompareAndSwap(false, true) {
		utils.WriteError(w, http.StatusConflict, "Já existe uma temporada sendo carregada")
		return
	}
	go func() {
		defer historyRunning.Store(false)
		// Contexto próprio: a carga continua depois que a resposta 202 sai
		if err := services.SyncFootballDataHistory(context.Background(), league, year); err != nil {
			utils.CustomLog("JOBS", "[%s %d] Erro ao carregar temporada antiga: %v", league, year, err)
			return
		}
		utils.CustomLog("JOBS", "[%s %d] Temporada antiga carregada", league, year)
	}()
	utils.WriteJSON(w, http.StatusAccepted, map[string]string{"message": "Carregando a temporada em background"})
}

// AdminSyncESPNSeasonHandler carrega na hora a temporada atual inteira de uma liga que vem
// da ESPN (o worker faz o mesmo toda madrugada), em background:
// POST /admin/sync-espn-season?league=UNL
func AdminSyncESPNSeasonHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Use POST")
		return
	}
	league := r.URL.Query().Get("league")
	espnOnly, err := database.GetESPNOnlyLeagues(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao listar as ligas")
		return
	}
	if !slices.Contains(espnOnly, league) {
		utils.WriteError(w, http.StatusBadRequest, "Informe uma liga que vem da ESPN: "+strings.Join(espnOnly, ", "))
		return
	}
	if !historyRunning.CompareAndSwap(false, true) {
		utils.WriteError(w, http.StatusConflict, "Já existe uma temporada sendo carregada")
		return
	}
	go func() {
		defer historyRunning.Store(false)
		// Contexto próprio: a carga continua depois que a resposta 202 sai
		days, err := services.SyncESPNSeason(context.Background(), league)
		if err != nil {
			utils.CustomLog("JOBS", "[%s] Erro ao carregar a temporada da ESPN: %v", league, err)
			return
		}
		utils.CustomLog("JOBS", "[%s] Temporada da ESPN carregada: %d dias com jogo", league, days)
	}()
	utils.WriteJSON(w, http.StatusAccepted, map[string]string{"message": "Carregando a temporada em background"})
}
