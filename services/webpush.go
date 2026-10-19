package services

import (
	"App-Futebol/database"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Notificações no navegador (Web Push, o padrão dos navegadores), para a versão web. O app
// usa o Expo; o site usa a inscrição do próprio navegador. A mensagem vai criptografada
// para o serviço de push do navegador (Google, Mozilla, Apple ou Microsoft), assinada com
// as chaves VAPID do servidor.
//
// VAPID_PUBLIC_KEY e VAPID_PRIVATE_KEY: o par de chaves (gere uma vez com -gerar-vapid).
// VAPID_SUBJECT: contato que os serviços de push usam se algo der errado (mailto: ou
// https:). Sem os três, o push no navegador fica desligado.

var errWebPushOff = errors.New("push no navegador não configurado (VAPID_PUBLIC_KEY, VAPID_PRIVATE_KEY e VAPID_SUBJECT)")

var webPushClient = &http.Client{Timeout: 15 * time.Second}

// WebPushPublicKey é a chave que o site usa para inscrever o navegador ("" = desligado)
func WebPushPublicKey() string {
	if !WebPushEnabled() {
		return ""
	}
	return os.Getenv("VAPID_PUBLIC_KEY")
}

func WebPushEnabled() bool {
	return os.Getenv("VAPID_PUBLIC_KEY") != "" && os.Getenv("VAPID_PRIVATE_KEY") != "" &&
		os.Getenv("VAPID_SUBJECT") != ""
}

// GenerateVAPIDKeys gera o par de chaves do servidor (uma vez só: trocar invalida as
// inscrições de todos os navegadores)
func GenerateVAPIDKeys() (privateKey, publicKey string, err error) {
	return webpush.GenerateVAPIDKeys()
}

// webPushPayload é o que o service worker do site recebe para mostrar a notificação
type webPushPayload struct {
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Tag   string            `json:"tag,omitempty"`
	Data  map[string]string `json:"data,omitempty"`
}

// sendWebPush manda o aviso para a inscrição de um navegador. gone = true quando o serviço
// de push diz que a inscrição não existe mais (404/410): a permissão foi tirada, o site
// foi removido ou o navegador trocou a inscrição.
func sendWebPush(ctx context.Context, t database.PushTarget, msg pushMessage) (gone bool, err error) {
	if !WebPushEnabled() {
		return false, errWebPushOff
	}
	payload, err := json.Marshal(webPushPayload{Title: msg.Title, Body: msg.Body, Tag: msg.Tag, Data: msg.Data})
	if err != nil {
		return false, err
	}
	ttl := msg.TTL
	if ttl == 0 {
		ttl = pushTTL
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload,
		&webpush.Subscription{Endpoint: t.Token, Keys: webpush.Keys{P256dh: t.P256dh, Auth: t.Auth}},
		&webpush.Options{
			HTTPClient: webPushClient,
			// A biblioteca põe o "mailto:" na frente de tudo que não for https:
			Subscriber:      strings.TrimPrefix(os.Getenv("VAPID_SUBJECT"), "mailto:"),
			VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
			VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
			TTL:             ttl,
			Urgency:         webpush.UrgencyHigh,
			// Mensagem ainda não entregue com o mesmo tópico é substituída (como o
			// collapseId do app); na tela, quem substitui é o tag
			Topic: msg.Tag,
		})
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return false, nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return true, fmt.Errorf("inscrição do navegador expirada (%s)", resp.Status)
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return false, fmt.Errorf("serviço de push respondeu %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
}
