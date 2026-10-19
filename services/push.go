package services

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Notificações push pelo Expo (https://docs.expo.dev/push-notifications/sending-notifications/).
// O Expo entrega no Android (pelo Firebase) e no iOS (pela Apple) com a mesma chamada,
// então o backend não guarda nenhuma chave do Firebase.

const (
	expoPushURL   = "https://exp.host/--/api/v2/push/send"
	expoBatchSize = 100 // limite de mensagens por requisição do Expo

	// Aviso que ficou na fila mais que isso não sai: se o worker ficou fora do ar,
	// um "GOL!" de meia hora atrás só atrapalha
	pushMaxAge = 15 * time.Minute
	// O Expo/Firebase descartam a notificação se o celular ficar offline mais que isso
	pushTTL = 3600
)

// Horário de Brasília (sem horário de verão desde 2019)
var brasilia = time.FixedZone("BRT", -3*60*60)

// ExpoMessage é uma notificação para um aparelho
type ExpoMessage struct {
	To       string            `json:"to"`
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Data     map[string]string `json:"data,omitempty"` // o app usa para abrir o jogo
	Sound    string            `json:"sound,omitempty"`
	Priority string            `json:"priority,omitempty"`
	TTL      int               `json:"ttl,omitempty"`

	// Mesmo valor nos avisos de um jogo: o novo substitui o que já está na tela, em vez
	// de empilhar uma notificação por gol. Tag faz isso no Android; CollapseID, no
	// iPhone (e no Android junta as que chegam com o celular offline)
	Tag        string `json:"tag,omitempty"`
	CollapseID string `json:"collapseId,omitempty"`
}

// ExpoTicket é a resposta do Expo para cada mensagem, na mesma ordem do envio
type ExpoTicket struct {
	Status  string `json:"status"` // ok | error
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
	Details struct {
		Error string `json:"error,omitempty"` // ex: DeviceNotRegistered
	} `json:"details"`
}

// EXPO_PUSH_URL troca o endereço do Expo (testes). EXPO_ACCESS_TOKEN só é necessário
// se a "segurança reforçada de push" estiver ligada no painel do Expo.
func expoURL() string {
	if u := os.Getenv("EXPO_PUSH_URL"); u != "" {
		return u
	}
	return expoPushURL
}

// SendExpo manda as mensagens em lotes de 100 e devolve um ticket por mensagem.
// Se um lote falhar, devolve os tickets dos lotes que já foram junto com o erro.
func SendExpo(ctx context.Context, msgs []ExpoMessage) ([]ExpoTicket, error) {
	tickets := make([]ExpoTicket, 0, len(msgs))
	for start := 0; start < len(msgs); start += expoBatchSize {
		batch, err := sendExpoBatch(ctx, msgs[start:min(start+expoBatchSize, len(msgs))])
		if err != nil {
			return tickets, err
		}
		tickets = append(tickets, batch...)
	}
	return tickets, nil
}

func sendExpoBatch(ctx context.Context, msgs []ExpoMessage) ([]ExpoTicket, error) {
	body, err := json.Marshal(msgs)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, expoURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token := os.Getenv("EXPO_ACCESS_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out struct {
		Data   []ExpoTicket `json:"data"`
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("resposta do Expo ilegível (HTTP %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || len(out.Errors) > 0 {
		msg := ""
		if len(out.Errors) > 0 {
			msg = out.Errors[0].Code + ": " + out.Errors[0].Message
		}
		return nil, fmt.Errorf("expo recusou o envio (HTTP %d) %s", resp.StatusCode, msg)
	}
	if len(out.Data) != len(msgs) {
		return nil, fmt.Errorf("expo devolveu %d tickets para %d mensagens", len(out.Data), len(msgs))
	}
	return out.Data, nil
}

// pushText monta o título e o texto de cada tipo de aviso
func pushText(p database.PendingPush) (string, string) {
	placar := fmt.Sprintf("%s %d x %d %s", p.HomeTeam, p.HomeScore, p.AwayScore, p.AwayTeam)

	switch p.Kind {
	case "start":
		return "Começou! ⚽", p.HomeTeam + " x " + p.AwayTeam

	case "goal":
		title := "⚽ GOLS!"
		switch p.GoalSide {
		case "home":
			title = "⚽ GOL! " + p.HomeTeam
		case "away":
			title = "⚽ GOL! " + p.AwayTeam
		}
		body := placar
		if p.Scorer != "" {
			autor := p.Scorer
			if p.OwnGoal {
				autor += " (contra)"
			}
			if p.Minute != "" {
				autor += " " + p.Minute
			}
			body += " · " + autor
		}
		return title, body

	case "red_card":
		title := "🟥 Cartão vermelho"
		switch p.GoalSide {
		case "home":
			title += " · " + p.HomeTeam
		case "away":
			title += " · " + p.AwayTeam
		}
		body := placar
		if p.Scorer != "" {
			expulso := p.Scorer
			if p.Minute != "" {
				expulso += " " + p.Minute
			}
			body = expulso + " · " + placar
		}
		return title, body

	case "end":
		if p.HomePenalty != nil && p.AwayPenalty != nil {
			placar = fmt.Sprintf("%s %d (%d) x (%d) %d %s", p.HomeTeam, p.HomeScore,
				*p.HomePenalty, *p.AwayPenalty, p.AwayScore, p.AwayTeam)
		}
		return "Fim de jogo", placar

	case "lineup":
		body := p.HomeTeam + " x " + p.AwayTeam
		if !p.MatchDate.IsZero() {
			body += " · começa às " + p.MatchDate.In(brasilia).Format("15:04")
		}
		return "Escalações confirmadas 📋", body
	}
	return "", ""
}

// pushData vai junto da notificação: o app usa para abrir a tela do jogo
func pushData(p database.PendingPush) map[string]string {
	data := map[string]string{
		"type":     p.Kind,
		"match_id": strconv.FormatInt(p.MatchID, 10),
		"league":   p.League,
	}
	if p.ESPNMatchID != 0 {
		data["espn_match_id"] = strconv.FormatInt(p.ESPNMatchID, 10)
	}
	if p.ESPNHomeID != 0 && p.ESPNAwayID != 0 {
		data["espn_home"] = strconv.FormatInt(p.ESPNHomeID, 10)
		data["espn_away"] = strconv.FormatInt(p.ESPNAwayID, 10)
	}
	if !p.MatchDate.IsZero() {
		data["date"] = p.MatchDate.UTC().Format(time.RFC3339)
	}
	return data
}

// matchPushTag identifica a notificação de um jogo: escalação, início, gols e fim do
// mesmo jogo ocupam uma notificação só, que vai sendo atualizada
func matchPushTag(matchID int64) string {
	return "jogo-" + strconv.FormatInt(matchID, 10)
}

// ProcessPushOutbox manda os avisos da fila. Roda no worker a cada 10 segundos.
func ProcessPushOutbox(ctx context.Context) {
	if n, err := database.ExpireStalePushes(ctx, pushMaxAge); err != nil {
		utils.CustomLog("PUSH", "Erro ao expirar avisos antigos: %v", err)
	} else if n > 0 {
		utils.CustomLog("PUSH", "%d aviso(s) expirado(s) sem envio", n)
	}

	pending, err := database.GetPendingPushes(ctx, 50)
	if err != nil {
		utils.CustomLog("PUSH", "Erro ao ler a fila de avisos: %v", err)
		return
	}
	for _, p := range pending {
		sendPending(ctx, p)
	}
}

func sendPending(ctx context.Context, p database.PendingPush) {
	targets, err := database.GetPushTargetsForMatch(ctx, p.MatchID, p.Kind)
	if err != nil {
		utils.CustomLog("PUSH", "Erro ao buscar quem segue o jogo %d: %v", p.MatchID, err)
		return // fica na fila e tenta de novo
	}
	if len(targets) == 0 {
		markSent(ctx, p, "sem seguidores (ou ninguém quer este tipo de aviso)")
		return
	}

	title, body := pushText(p)
	msg := pushMessage{Title: title, Body: body, Data: pushData(p), Tag: matchPushTag(p.MatchID), TTL: pushTTL}
	delivered, partial, retry := deliver(ctx, targets, msg)
	if retry {
		utils.CustomLog("PUSH", "Nenhum serviço de push aceitou o aviso %d (%s): fica na fila", p.ID, p.Kind)
		return // nada saiu: fica na fila até expirar
	}

	result := fmt.Sprintf("ok %d/%d", delivered, len(targets))
	if partial != "" {
		result += " (parcial: " + partial + ")"
	}
	markSent(ctx, p, result)
	utils.CustomLog("PUSH", "%s | %s — %s [%s]", title, body, p.League, result)
}

// pushMessage é um aviso, igual para o app (Expo) e para o site (Web Push)
type pushMessage struct {
	Title, Body string
	Data        map[string]string
	Tag         string // mesmo valor = substitui o aviso anterior na tela
	TTL         int
}

// deliver manda o aviso para os aparelhos: app pelo Expo, navegador por Web Push.
// Inscrição que não existe mais é apagada. Devolve quantos receberam; partial descreve a
// falha de transporte quando parte saiu (não reenvia, para ninguém receber duas vezes);
// retry = true quando nada saiu por falha de transporte (vale tentar de novo).
func deliver(ctx context.Context, targets []database.PushTarget, msg pushMessage) (delivered int, partial string, retry bool) {
	var expo []database.PushTarget
	var web []database.PushTarget
	for _, t := range targets {
		if t.IsWeb() {
			web = append(web, t)
		} else {
			expo = append(expo, t)
		}
	}
	var transportErrs []string

	if len(expo) > 0 {
		msgs := make([]ExpoMessage, len(expo))
		for i, t := range expo {
			msgs[i] = ExpoMessage{To: t.Token, Title: msg.Title, Body: msg.Body, Data: msg.Data,
				Sound: "default", Priority: "high", TTL: msg.TTL, Tag: msg.Tag, CollapseID: msg.Tag}
		}
		tickets, err := SendExpo(ctx, msgs)
		if err != nil {
			transportErrs = append(transportErrs, err.Error())
		}
		for i, t := range tickets {
			if t.Status == "ok" {
				delivered++
				continue
			}
			if t.Details.Error == "DeviceNotRegistered" && i < len(expo) {
				// App desinstalado ou notificação desligada: para de mandar para esse aparelho
				database.DeletePushToken(ctx, expo[i].Token)
			}
			utils.CustomLog("PUSH", "Aviso recusado para um aparelho: %s %s", t.Details.Error, t.Message)
		}
	}

	for _, t := range web {
		gone, err := sendWebPush(ctx, t, msg)
		switch {
		case err == nil:
			delivered++
		case gone:
			database.DeletePushToken(ctx, t.Token)
			utils.CustomLog("PUSH", "Navegador removido: %v", err)
		default:
			transportErrs = append(transportErrs, "web: "+err.Error())
		}
	}

	if len(transportErrs) == 0 {
		return delivered, "", false
	}
	if delivered == 0 {
		return 0, "", true
	}
	return delivered, strings.Join(transportErrs, "; "), false
}

func markSent(ctx context.Context, p database.PendingPush, result string) {
	if err := database.MarkPushSent(ctx, p.ID, result); err != nil {
		utils.CustomLog("PUSH", "Erro ao tirar o aviso %d da fila: %v", p.ID, err)
	}
}

// SendTestPush manda uma notificação de teste para um aparelho (rota de admin)
// SendTestPush manda o aviso de teste para um aparelho (app ou navegador). No navegador, o
// resultado vem no mesmo formato do Expo: Status "ok" ou "error", e Details.Error =
// "DeviceNotRegistered" quando a inscrição não existe mais.
func SendTestPush(ctx context.Context, target database.PushTarget) (ExpoTicket, error) {
	msg := pushMessage{
		Title: "Notificações ligadas ✅",
		Body:  "Se você está vendo isto, o push do FutStatis está funcionando.",
		Data:  map[string]string{"type": "test"},
		TTL:   pushTTL,
	}
	if target.IsWeb() {
		gone, err := sendWebPush(ctx, target, msg)
		switch {
		case err == nil:
			return ExpoTicket{Status: "ok"}, nil
		case gone:
			tk := ExpoTicket{Status: "error", Message: err.Error()}
			tk.Details.Error = "DeviceNotRegistered"
			return tk, nil
		}
		return ExpoTicket{}, err
	}

	tickets, err := SendExpo(ctx, []ExpoMessage{{
		To: target.Token, Title: msg.Title, Body: msg.Body, Data: msg.Data,
		Sound: "default", Priority: "high",
	}})
	if err != nil {
		return ExpoTicket{}, err
	}
	return tickets[0], nil
}

// NotifyAdmins manda um aviso para os aparelhos de admin (app ou navegador), sem passar
// pela fila: alertas do próprio servidor
func NotifyAdmins(ctx context.Context, title, body string) {
	targets, err := database.GetAdminPushTargets(ctx)
	if err != nil || len(targets) == 0 {
		return
	}
	deliver(ctx, targets, pushMessage{Title: title, Body: body,
		Data: map[string]string{"type": "admin_alert"}, Tag: "admin-alert", TTL: 24 * 3600})
}
