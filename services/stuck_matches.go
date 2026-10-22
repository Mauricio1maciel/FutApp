package services

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"context"
	"fmt"
	"strconv"
	"time"
)

// Aviso de jogo sem resultado. Uma fonte às vezes para (a football-data parou a
// Libertadores de 2026 em 22/09) e o jogo continua "marcado" depois de acabar. Quem
// percebia era o admin, olhando o app. Agora o worker procura esses jogos, tenta trazer o
// resultado de novo pela ESPN e avisa o admin dos que continuarem sem resultado, uma vez
// por jogo (a marca fica em alert_state, então reiniciar o worker não repete o aviso).

const stuckAlertPrefix = "jogo_sem_resultado:"

// resyncESPNDay busca de novo o placar de um dia na ESPN (o trigger leva para os jogos).
// Variável para os testes não chamarem a ESPN de verdade
var resyncESPNDay = SyncESPNScoreboardForDate

// CheckStuckMatches procura os jogos que já deviam ter acabado, pede de novo à ESPN o
// placar dos dias deles e avisa o admin dos que continuarem sem resultado
func CheckStuckMatches(ctx context.Context) error {
	if err := database.DeleteOldAlertStates(ctx, stuckAlertPrefix, 7*24*time.Hour); err != nil {
		utils.CustomLog("JOBS", "Erro ao limpar os avisos antigos de jogo sem resultado: %v", err)
	}
	stuck, err := database.GetStuckMatches(ctx)
	if err != nil || len(stuck) == 0 {
		return err
	}

	// Um pedido por liga e dia (o placar do dia traz todos os jogos dele)
	retried := map[string]bool{}
	for _, m := range stuck {
		if key := m.League + "|" + m.ESPNDay; !retried[key] {
			retried[key] = true
			resyncESPNDay(ctx, m.League, m.ESPNDay)
		}
	}
	if stuck, err = database.GetStuckMatches(ctx); err != nil || len(stuck) == 0 {
		return err
	}

	var fresh []database.StuckMatch
	for _, m := range stuck {
		_, alerted, err := database.GetAlertState(ctx, stuckAlertKey(m))
		if err != nil {
			return err
		}
		if !alerted {
			fresh = append(fresh, m)
		}
	}
	utils.CustomLog("JOBS", "%d jogo(s) sem resultado depois de buscar de novo na ESPN (%d ainda não avisado(s))", len(stuck), len(fresh))
	if len(fresh) == 0 {
		return nil
	}
	for _, m := range fresh {
		utils.CustomLog("JOBS", "Sem resultado: %s", stuckMatchText(m))
	}

	targets, err := database.GetAdminPushTargets(ctx)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		// Não marca: o aviso sai quando houver um aparelho
		utils.CustomLog("JOBS", "Nenhum aparelho recebe os alertas: toque em \"Enviar push de teste para este aparelho\" na tela de admin do app")
		return nil
	}
	title, body := stuckAlertText(fresh)
	delivered, _, _ := deliver(ctx, targets, pushMessage{
		Title: title, Body: body,
		Data: map[string]string{"type": "admin_alert", "alert": "stuck_matches"},
		Tag:  "admin-stuck-matches", TTL: 24 * 3600,
	})
	if delivered == 0 {
		return fmt.Errorf("nenhum aparelho de admin recebeu o aviso de jogo sem resultado")
	}
	for _, m := range fresh {
		if err := database.SetAlertState(ctx, stuckAlertKey(m), time.Now().Unix()); err != nil {
			return err
		}
	}
	return nil
}

func stuckAlertKey(m database.StuckMatch) string {
	return stuckAlertPrefix + strconv.FormatInt(m.MatchID, 10)
}

// stuckMatchText: "Palmeiras x Fluminense (CLI, 21/10 21:30): na ESPN acabou 0 x 1, mas o
// resultado não chegou ao jogo"
func stuckMatchText(m database.StuckMatch) string {
	text := fmt.Sprintf("%s x %s (%s, %s)", m.HomeTeam, m.AwayTeam, m.League, m.MatchDate.In(brasilia).Format("02/01 15:04"))
	switch m.ESPNStatus {
	case "post":
		// O trigger não achou o jogo: vínculo dos times com a ESPN ou data diferente
		return text + fmt.Sprintf(": na ESPN acabou %s x %s, mas o resultado não chegou ao jogo", m.ESPNHome, m.ESPNAway)
	case "":
		return text + ": a ESPN não tem o jogo (confira o vínculo dos times e a data)"
	}
	return text + ": a ESPN também não atualizou"
}

func stuckAlertText(ms []database.StuckMatch) (title, body string) {
	title = "⚠️ Jogo sem resultado"
	body = stuckMatchText(ms[0])
	if len(ms) > 1 {
		title = fmt.Sprintf("⚠️ %d jogos sem resultado", len(ms))
		body += fmt.Sprintf(". E mais %d", len(ms)-1)
	}
	return title, body + "."
}
