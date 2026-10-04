package worker

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"App-Futebol/utils"
	"sync/atomic"
	"time"
)

// Rotinas de sincronização com as APIs externas. Com elas, a API web só lê do banco.
//
//   - Jogos da football-data: uma liga por minuto (limite do plano grátis: 10 req/min),
//     seguida do recálculo da classificação dessa liga
//   - Jogos passados sem detalhes da ESPN: a cada 10 minutos
//   - Artilharia/assistências da ESPN: a cada 6 horas
//   - 03:00: times e elencos da football-data, vínculo com a ESPN e elencos da ESPN
func startDataJobs() {
	go loopFootballDataMatches()
	go loopMissingMatches()
	go loopLeagueStats()
	go loopDaily()
}

func loopFootballDataMatches() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	index := 0
	for ; ; <-ticker.C {
		league := services.FootballDataLeagues[index]
		index = (index + 1) % len(services.FootballDataLeagues)

		if err := services.SyncFootballDataMatches(league); err != nil {
			utils.CustomLog("JOBS", "[%s] Erro na football-data: %v", league, err)
		} else {
			recalculateStandings(league)
		}

		// A UNL não vem da football-data: os jogos chegam pela ESPN (trigger),
		// então recalcula junto com o fim de cada volta pelas ligas
		if index == 0 {
			recalculateStandings("UNL")
		}
	}
}

func recalculateStandings(league string) {
	season := services.ResolveSeason(league, "")
	if err := services.RecalculateStandings(league, season); err != nil {
		utils.CustomLog("JOBS", "[%s] Erro ao recalcular classificação: %v", league, err)
	}
}

func loopMissingMatches() {
	for {
		for _, league := range services.FootballDataLeagues {
			missing, err := database.GetMissingMatches(league)
			if err != nil {
				continue
			}
			for _, m := range missing {
				services.UpdateMatchFromESPN(m["home"], m["away"], m["date"], league)
				time.Sleep(5 * time.Second)
			}
		}
		time.Sleep(10 * time.Minute)
	}
}

func loopLeagueStats() {
	for {
		for league := range services.ESPNLeagueMap {
			services.SyncLeagueStatsBackground(league, services.ResolveSeason(league, ""))
			time.Sleep(5 * time.Second)
		}
		time.Sleep(6 * time.Hour)
	}
}

func loopDaily() {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
		if now.After(next) {
			next = next.Add(24 * time.Hour)
		}
		utils.CustomLog("JOBS", "Próxima sincronização diária: %v", next)
		time.Sleep(time.Until(next))

		StartDailySync()
	}
}

// RunDailySync atualiza times, elencos e vínculos com a ESPN.
// Também pode ser disparado pelo admin em /admin/sync-daily.
func RunDailySync() {
	utils.CustomLog("JOBS", "Iniciando sincronização diária...")

	for _, league := range services.FootballDataLeagues {
		if err := services.SyncFootballDataTeams(league); err != nil {
			utils.CustomLog("JOBS", "[%s] Erro ao sincronizar times: %v", league, err)
		}
		time.Sleep(15 * time.Second) // 2 chamadas por liga, folga no limite de 10/min
	}

	if _, err := services.SyncESPNTeamLinks(); err != nil {
		utils.CustomLog("JOBS", "Erro ao vincular times à ESPN: %v", err)
	}

	services.SyncAllESPNRosters()

	utils.CustomLog("JOBS", "Sincronização diária finalizada!")
}

var dailySyncRunning atomic.Bool

// StartDailySync dispara a sincronização diária em background, se ela ainda não estiver rodando
func StartDailySync() bool {
	if !dailySyncRunning.CompareAndSwap(false, true) {
		return false
	}
	go func() {
		defer dailySyncRunning.Store(false)
		RunDailySync()
	}()
	return true
}
