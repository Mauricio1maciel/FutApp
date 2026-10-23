package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/services"
	"App-Futebol/utils"
	"context"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

func MatchesHandler(w http.ResponseWriter, r *http.Request) {

	league := r.URL.Query().Get("league")
	roundStr := r.URL.Query().Get("round")
	stageStr := r.URL.Query().Get("stage")
	dateStr := r.URL.Query().Get("date")
	season := r.URL.Query().Get("season")
	// Só o admin pode forçar a busca na football-data; para os demais, o worker mantém o banco atualizado
	forceUpdate := r.URL.Query().Get("update") == "true" && middlewares.IsAdmin(r)

	if league == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe a liga (ex: BSA, PL, PD)")
		return
	}
	if forceUpdate {
		// Em segundo plano: a atualização de uma liga leva mais que o tempo-limite do app
		// (15 s), e o pedido cancelado interrompia a gravação no meio. A resposta sai na
		// hora, com os dados atuais; os novos aparecem em cerca de um minuto
		state := "running"
		if startLeagueUpdate(league) {
			state = "started"
			utils.CustomLog("API", "Admin pediu a atualização dos jogos: %s", league)
		}
		w.Header().Set("X-Update", state)
	}

	season = services.ResolveSeason(r.Context(), league, season)

	filter := database.MatchFilter{Date: dateStr}
	switch {
	case dateStr != "":
		// Só a data
	case roundStr != "" || stageStr != "":
		// O app escolheu: round número = rodada, round texto = fase (ex: SEMI_FINALS).
		// stage + round número juntos separam rodadas que se repetem entre fases: na
		// Champions, round=1 é a 1ª rodada da fase de liga e também os jogos de ida do
		// mata-mata (stage=LEAGUE_STAGE&round=1 traz só a rodada da fase de liga)
		filter.Stage = stageStr
		if n, err := strconv.Atoi(roundStr); err == nil {
			filter.Round = &n
		} else if roundStr != "" {
			filter.Stage = roundStr
		}
	default:
		// Sem round: a fase e a rodada atuais, do mesmo jeito para ligas e copas.
		// Mata-mata (rodada 0) traz a fase inteira; liga e fase de grupos, a rodada.
		// Sem fase gravada (temporadas antigas), filtra sempre pela rodada, para
		// nunca devolver a temporada inteira.
		stage, round, err := services.CurrentStage(r.Context(), league, season)
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar jogos")
			return
		}
		filter.Stage = stage
		if round > 0 || stage == "" {
			filter.Round = &round
		}
	}

	matches, err := database.GetMatchesByLeague(r.Context(), league, season, filter)
	if err != nil {
		log.Printf("Erro ao buscar jogos no banco: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar jogos")
		return
	}
	utils.WriteJSON(w, http.StatusOK, matches)
}

// Atualizações de liga pedidas pelo admin em andamento (uma por liga de cada vez)
var leagueUpdates sync.Map

// updateLeague roda a atualização (variável para os testes trocarem)
var updateLeague = func(league string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	services.UpdateLeagueNow(ctx, league)
}

// startLeagueUpdate dispara a atualização da liga em segundo plano. false = já havia uma
// rodando para essa liga.
func startLeagueUpdate(league string) bool {
	if _, running := leagueUpdates.LoadOrStore(league, true); running {
		return false
	}
	go func() {
		defer leagueUpdates.Delete(league)
		updateLeague(league)
	}()
	return true
}
