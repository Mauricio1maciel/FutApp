package worker

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"App-Futebol/utils"
	"context"
	"sync/atomic"
	"time"
)

// Rotinas de sincronização com as APIs externas. Com elas, a API web só lê do banco.
//
//   - Jogos da football-data: uma liga por minuto (limite do plano grátis: 10 req/min),
//     seguida do recálculo da classificação dessa liga
//   - Jogos passados sem detalhes da ESPN: a cada 10 minutos
//   - Artilharia/assistências da ESPN: a cada 6 horas
//   - 03:00: times e elencos da football-data, vínculo com a ESPN, elencos da ESPN e os
//     próximos 14 dias das ligas que vêm só da ESPN (Sul-Americana, Liga das Nações)
//   - Notificações push: a fila de avisos é lida a cada 10 segundos
//   - API do Render: /health a cada 10 minutos, para ela não dormir (API_PUBLIC_URL)
func startDataJobs() {
	go loopFootballDataMatches()
	go loopMissingMatches()
	go loopLeagueStats()
	go loopDaily()
	go loopPushNotifications()
	go loopKeepAPIAwake()
}

func loopFootballDataMatches() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	index := 0
	for ; ; <-ticker.C {
		ctx := context.Background()
		league := services.FootballDataLeagues[index]
		index = (index + 1) % len(services.FootballDataLeagues)

		if err := services.SyncFootballDataMatches(ctx, league); err != nil {
			utils.CustomLog("JOBS", "[%s] Erro na football-data: %v", league, err)
		} else {
			recalculateStandings(ctx, league)
		}

		// As ligas que vêm só da ESPN (UNL, CSU) não passam pela football-data: os
		// jogos chegam pelo trigger, então recalcula no fim de cada volta pelas ligas
		if index == 0 {
			espnOnly, err := database.GetESPNOnlyLeagues(ctx)
			if err != nil {
				utils.CustomLog("JOBS", "Erro ao listar ligas só da ESPN: %v", err)
			}
			for _, l := range espnOnly {
				recalculateStandings(ctx, l)
			}
		}
	}
}

func recalculateStandings(ctx context.Context, league string) {
	season := services.ResolveSeason(ctx, league, "")
	if err := services.RecalculateStandings(ctx, league, season); err != nil {
		utils.CustomLog("JOBS", "[%s] Erro ao recalcular classificação: %v", league, err)
	}
}

func loopMissingMatches() {
	for {
		ctx := context.Background()
		for _, league := range services.FootballDataLeagues {
			missing, err := database.GetMissingMatches(ctx, league)
			if err != nil {
				continue
			}
			for _, m := range missing {
				services.UpdateMatchFromESPN(ctx, m["home"], m["away"], m["date"], league)
				time.Sleep(5 * time.Second)
			}
		}
		time.Sleep(10 * time.Minute)
	}
}

func loopLeagueStats() {
	for {
		ctx := context.Background()
		for league := range services.ESPNLeagueMap {
			services.SyncLeagueStatsBackground(ctx, league, services.ResolveSeason(ctx, league, ""))
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
	ctx := context.Background()

	for _, league := range services.FootballDataLeagues {
		if err := services.SyncFootballDataTeams(ctx, league); err != nil {
			utils.CustomLog("JOBS", "[%s] Erro ao sincronizar times: %v", league, err)
		}
		time.Sleep(15 * time.Second) // 2 chamadas por liga, folga no limite de 10/min
	}

	if _, err := services.SyncESPNTeamLinks(ctx); err != nil {
		utils.CustomLog("JOBS", "Erro ao vincular times à ESPN: %v", err)
	}

	services.SyncAllESPNRosters(ctx)

	syncUpcomingESPNOnly(ctx)

	utils.CustomLog("JOBS", "Sincronização diária finalizada!")
}

// syncUpcomingESPNOnly traz os jogos de ontem até daqui a 14 dias das ligas que vêm só
// da ESPN. O worker só busca o placar do dia: sem isso, os próximos jogos da Sul-Americana
// e da Liga das Nações só apareceriam no app no próprio dia (e quem segue o time não
// veria o jogo marcado).
func syncUpcomingESPNOnly(ctx context.Context) {
	leagues, err := database.GetESPNOnlyLeagues(ctx)
	if err != nil {
		utils.CustomLog("JOBS", "Erro ao listar ligas só da ESPN: %v", err)
		return
	}
	today := time.Now()
	for _, league := range leagues {
		for d := -1; d <= 14; d++ {
			services.SyncESPNScoreboardForDate(ctx, league, today.AddDate(0, 0, d).Format("20060102"))
			time.Sleep(time.Second) // Não martelar a ESPN
		}
	}
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
