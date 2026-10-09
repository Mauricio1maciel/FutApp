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
	tokens, err := database.GetPushTokensForMatch(ctx, p.MatchID)
	if err != nil {
		utils.CustomLog("PUSH", "Erro ao buscar quem segue o jogo %d: %v", p.MatchID, err)
		return // fica na fila e tenta de novo
	}
	if len(tokens) == 0 {
		markSent(ctx, p, "sem seguidores")
		return
	}

	title, body := pushText(p)
	data := pushData(p)
	msgs := make([]ExpoMessage, len(tokens))
	for i, t := range tokens {
		msgs[i] = ExpoMessage{To: t, Title: title, Body: body, Data: data,
			Sound: "default", Priority: "high", TTL: pushTTL}
	}

	tickets, err := SendExpo(ctx, msgs)
	if err != nil && len(tickets) == 0 {
		utils.CustomLog("PUSH", "Expo falhou no aviso %d (%s): %v", p.ID, p.Kind, err)
		return // nada saiu: fica na fila até expirar
	}

	ok := 0
	for i, t := range tickets {
		if t.Status == "ok" {
			ok++
			continue
		}
		if t.Details.Error == "DeviceNotRegistered" {
			// App desinstalado ou notificação desligada: para de mandar para esse aparelho
			database.DeletePushToken(ctx, tokens[i])
		}
		utils.CustomLog("PUSH", "Aviso %d recusado para um aparelho: %s %s", p.ID, t.Details.Error, t.Message)
	}

	result := fmt.Sprintf("ok %d/%d", ok, len(tokens))
	if err != nil {
		// Parte dos lotes saiu: não reenvia, para quem já recebeu não receber de novo
		result += " (parcial: " + err.Error() + ")"
	}
	markSent(ctx, p, result)
	utils.CustomLog("PUSH", "%s | %s — %s [%s]", title, body, p.League, result)
}

func markSent(ctx context.Context, p database.PendingPush, result string) {
	if err := database.MarkPushSent(ctx, p.ID, result); err != nil {
		utils.CustomLog("PUSH", "Erro ao tirar o aviso %d da fila: %v", p.ID, err)
	}
}

// SendTestPush manda uma notificação de teste para um aparelho (rota de admin)
func SendTestPush(ctx context.Context, token string) (ExpoTicket, error) {
	tickets, err := SendExpo(ctx, []ExpoMessage{{
		To:       token,
		Title:    "Notificações ligadas ✅",
		Body:     "Se você está vendo isto, o push do FutStatis está funcionando.",
		Data:     map[string]string{"type": "test"},
		Sound:    "default",
		Priority: "high",
	}})
	if err != nil {
		return ExpoTicket{}, err
	}
	return tickets[0], nil
}
