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
//   - 03:00: times e elencos da football-data, vínculo com a ESPN, elencos da ESPN, a
//     temporada inteira das ligas que vêm só da ESPN (Série B, Copa do Brasil,
//     Sul-Americana, Liga das Nações), pelo calendário da ESPN, e a limpeza das narrações
//     (mais de 30 dias) e dos vídeos (mais de 90)
//   - Vídeos e narração dos jogos encerrados nas últimas 12 horas: a cada 30 minutos (os
//     melhores momentos saem horas depois do apito)
//   - Notificações push: a fila de avisos é lida a cada 10 segundos
//   - API do Render: /health a cada 10 minutos, para ela não dormir (API_PUBLIC_URL)
//   - Tamanho do banco: ao subir e todo dia ao meio-dia; push para o admin quando passar
//     de DB_SIZE_ALERT_MB (padrão 300) e a cada 50 MB a mais
//   - Sinal de vida a um monitor externo a cada 5 minutos (HEALTHCHECK_URL)
//   - 04:00: backup do banco no Orange Pi, mantendo as últimas 7 cópias
//   - Jogos que já deviam ter acabado e continuam sem resultado: a cada 30 minutos busca
//     o dia de novo na ESPN e avisa o admin dos que continuarem assim
//
// Cada laço roda sob supervise (worker/protect.go): um pânico é registrado e o laço
// recomeça, em vez de derrubar o worker inteiro.
func startDataJobs() {
	go supervise("football-data", loopFootballDataMatches)
	go supervise("jogos sem detalhes", loopMissingMatches)
	go supervise("artilharia", loopLeagueStats)
	go supervise("rotina diária", loopDaily)
	go supervise("avisos push", loopPushNotifications)
	go supervise("API acordada", loopKeepAPIAwake)
	go supervise("tamanho do banco", loopDatabaseSize)
	go supervise("sinal de vida", loopHealthcheck)
	go supervise("backup", loopBackup)
	go protect("chaves do push no navegador", checkWebPushKeys)
	go supervise("jogos sem resultado", loopStuckMatches)
	go supervise("vídeos dos jogos", loopMatchMedia)
}

// loopMatchMedia busca de novo, a cada 30 minutos, os vídeos e a narração dos jogos
// encerrados nas últimas 12 horas (services.RefreshRecentMatchMedia)
func loopMatchMedia() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		if n, err := services.RefreshRecentMatchMedia(ctx); err != nil {
			utils.CustomLog("JOBS", "Erro ao buscar os vídeos dos jogos: %v", err)
		} else if n > 0 {
			utils.CustomLog("JOBS", "Vídeos novos em %d jogo(s) encerrado(s)", n)
		}
		cancel()
		time.Sleep(30 * time.Minute)
	}
}

func loopFootballDataMatches() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	index := 0
	for ; ; <-ticker.C {
		ctx := context.Background()
		league := services.FootballDataLeagues[index]
		index = (index + 1) % len(services.FootballDataLeagues)

		// Sinal de vida: a volta aconteceu (a football-data fora do ar não é travamento)
		lastDataRun.Store(time.Now().Unix())
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

	syncESPNOnlySeasons(ctx)

	// Rede de segurança das ligas da football-data: ela pode ficar parada e o placar ao
	// vivo só olha o dia de hoje. Reconfere na ESPN os últimos 3 dias de cada uma
	for _, league := range services.FootballDataLeagues {
		services.SyncESPNRecentDays(ctx, league, 3)
	}

	// Lance a lance só dos últimos 30 dias e vídeos dos últimos 90: ocupam espaço e
	// interessam no jogo e logo depois
	if c, v, err := services.ClearOldMatchMedia(ctx); err != nil {
		utils.CustomLog("JOBS", "Erro ao apagar narrações e vídeos antigos: %v", err)
	} else if c+v > 0 {
		utils.CustomLog("JOBS", "Apagados: narração de %d jogos (mais de 30 dias) e vídeos de %d (mais de 90)", c, v)
	}

	utils.CustomLog("JOBS", "Sincronização diária finalizada!")
}

// syncESPNOnlySeasons confere a temporada inteira das ligas que vêm só da ESPN, pelo
// calendário dela. O worker ao vivo só olha o placar do dia: sem isso, os próximos jogos
// só apareceriam no app no próprio dia, uma liga nova ficaria sem as rodadas anteriores e
// os jogos de quando o worker esteve desligado ficariam sem resultado.
func syncESPNOnlySeasons(ctx context.Context) {
	leagues, err := database.GetESPNOnlyLeagues(ctx)
	if err != nil {
		utils.CustomLog("JOBS", "Erro ao listar ligas só da ESPN: %v", err)
		return
	}
	for _, league := range leagues {
		days, err := services.SyncESPNSeason(ctx, league)
		if err == nil {
			utils.CustomLog("JOBS", "[%s] Temporada conferida: %d dias com jogo", league, days)
			continue
		}
		// Sem o calendário, ao menos a última semana e as duas próximas
		utils.CustomLog("JOBS", "[%s] Calendário da ESPN indisponível (%v): conferindo de 7 dias atrás a 14 à frente", league, err)
		today := time.Now()
		for d := -7; d <= 14; d++ {
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
		protect("sincronização diária", RunDailySync)
	}()
	return true
}
