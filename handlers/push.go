package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/services"
	"App-Futebol/utils"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Rotas de notificação push. O aparelho é identificado pelo device_id do token de
// convidado: o app chama estas rotas com o token de convidado, mesmo logado como admin.

// flexID aceita o ID como número ou texto no JSON (123 ou "123"): o app guarda alguns
// IDs como texto, como o id_event da ESPN
type flexID int64

func (f *flexID) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return fmt.Errorf("ID inválido: %s", s)
	}
	*f = flexID(n)
	return nil
}

func pushDevice(w http.ResponseWriter, r *http.Request) (string, bool) {
	device := middlewares.DeviceID(r)
	if device == "" {
		utils.WriteError(w, http.StatusBadRequest, "Use um token com device_id (de convidado, ou da conta com device_id no login) nas rotas /push")
		return "", false
	}
	return device, true
}

// subscriptionOwner: quem segue. Com conta, a conta (vale em todos os aparelhos dela); sem,
// o aparelho do token de convidado
func subscriptionOwner(w http.ResponseWriter, r *http.Request) (database.Owner, bool) {
	if user := middlewares.UserID(r); user != 0 {
		return database.User(user), true
	}
	device, ok := pushDevice(w, r)
	return database.Device(device), ok
}

// linkDeviceToAccount liga o aparelho que acabou de registrar o push à conta do token
func linkDeviceToAccount(r *http.Request, device string) {
	if user := middlewares.UserID(r); user != 0 {
		if err := database.SetDeviceUser(r.Context(), device, user); err != nil {
			utils.CustomLog("PUSH", "Erro ao ligar o aparelho à conta %d: %v", user, err)
		}
	}
}

func decodeBody(r *http.Request, v interface{}) error {
	return json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(v)
}

func isExpoToken(t string) bool {
	return len(t) < 256 && strings.HasSuffix(t, "]") &&
		(strings.HasPrefix(t, "ExponentPushToken[") || strings.HasPrefix(t, "ExpoPushToken["))
}

// PushRegisterHandler: POST registra o token de push do aparelho; DELETE desliga as
// notificações e apaga tudo o que o aparelho seguia
func PushRegisterHandler(w http.ResponseWriter, r *http.Request) {
	device, ok := pushDevice(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodPost:
		var body struct {
			Token        string           `json:"token"`
			Platform     string           `json:"platform"`
			Subscription *webSubscription `json:"subscription"`
		}
		if err := decodeBody(r, &body); err != nil {
			utils.WriteError(w, http.StatusBadRequest, "JSON inválido")
			return
		}
		if body.Platform == "web" {
			registerWebPush(w, r, device, body.Subscription)
			return
		}
		if !isExpoToken(body.Token) {
			utils.WriteError(w, http.StatusBadRequest, "Token inválido: esperado ExponentPushToken[...]")
			return
		}
		if body.Platform != "" && body.Platform != "android" && body.Platform != "ios" {
			utils.WriteError(w, http.StatusBadRequest, "platform deve ser android, ios ou web")
			return
		}
		if err := database.RegisterPushDevice(r.Context(), device, body.Token, body.Platform); err != nil {
			utils.CustomLog("PUSH", "Erro ao registrar aparelho: %v", err)
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao registrar o aparelho")
			return
		}
		linkDeviceToAccount(r, device)
		utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	case http.MethodDelete:
		if err := database.UnregisterPushDevice(r.Context(), device); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao desligar as notificações")
			return
		}
		utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	default:
		utils.WriteError(w, http.StatusMethodNotAllowed, "Use POST ou DELETE")
	}
}

// PushSubscriptionsHandler: GET lista o que a conta (ou, sem conta, o aparelho) segue; POST passa a seguir um
// jogo, um time ou uma liga; DELETE deixa de seguir
func PushSubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	owner, ok := subscriptionOwner(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		subs, err := database.GetPushSubscriptions(r.Context(), owner)
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar o que você segue")
			return
		}
		utils.WriteJSON(w, http.StatusOK, subs)

	case http.MethodPost:
		var body struct {
			MatchID     flexID    `json:"match_id"`
			ESPNMatchID flexID    `json:"espn_match_id"`
			TeamID      flexID    `json:"team_id"`
			League      string    `json:"league"`
			Kinds       *[]string `json:"kinds"`
		}
		if err := decodeBody(r, &body); err != nil {
			utils.WriteError(w, http.StatusBadRequest, "JSON inválido")
			return
		}
		kinds, setKinds, msg := parseKinds(body.Kinds)
		if msg != "" {
			utils.WriteError(w, http.StatusBadRequest, msg)
			return
		}
		target, ok := resolveTarget(w, r, int64(body.MatchID), int64(body.ESPNMatchID), int64(body.TeamID), body.League)
		if !ok {
			return
		}

		var err error
		switch target.kind {
		case "team":
			err = database.SubscribeTeam(r.Context(), owner, target.id, kinds, setKinds)
		case "league":
			err = database.SubscribeLeague(r.Context(), owner, target.league, kinds, setKinds)
		default:
			err = database.SubscribeMatch(r.Context(), owner, target.id, kinds, setKinds)
		}
		switch {
		case errors.Is(err, database.ErrDeviceNotRegistered):
			utils.WriteError(w, http.StatusConflict, "Registre o aparelho em POST /push/register antes de seguir")
		case errors.Is(err, database.ErrNotFound):
			utils.WriteError(w, http.StatusNotFound, target.label()+" não encontrado")
		case err != nil:
			utils.CustomLog("PUSH", "Erro ao seguir: %v", err)
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao seguir")
		default:
			resp := target.response()
			if setKinds {
				if kinds == nil {
					kinds = database.PushKinds
				}
				resp["kinds"] = kinds
			}
			utils.WriteJSON(w, http.StatusOK, resp)
		}

	case http.MethodDelete:
		q := r.URL.Query()
		matchID, _ := strconv.ParseInt(q.Get("match_id"), 10, 64)
		espnID, _ := strconv.ParseInt(q.Get("espn_match_id"), 10, 64)
		teamID, _ := strconv.ParseInt(q.Get("team_id"), 10, 64)
		target, ok := resolveTarget(w, r, matchID, espnID, teamID, q.Get("league"))
		if !ok {
			return
		}

		var err error
		switch target.kind {
		case "team":
			err = database.UnsubscribeTeam(r.Context(), owner, target.id)
		case "league":
			err = database.UnsubscribeLeague(r.Context(), owner, target.league)
		default:
			err = database.UnsubscribeMatch(r.Context(), owner, target.id)
		}
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao deixar de seguir")
			return
		}
		utils.WriteJSON(w, http.StatusOK, target.response())

	default:
		utils.WriteError(w, http.StatusMethodNotAllowed, "Use GET, POST ou DELETE")
	}
}

type subscriptionTarget struct {
	kind   string // match | team | league
	id     int64
	league string
}

func (t subscriptionTarget) label() string {
	switch t.kind {
	case "team":
		return "Time"
	case "league":
		return "Liga"
	}
	return "Jogo"
}

// response devolve o que foi seguido: quem segue pelo ID da ESPN fica sabendo o match_id
func (t subscriptionTarget) response() map[string]interface{} {
	switch t.kind {
	case "team":
		return map[string]interface{}{"status": "ok", "team_id": t.id}
	case "league":
		return map[string]interface{}{"status": "ok", "league": t.league}
	}
	return map[string]interface{}{"status": "ok", "match_id": t.id}
}

// resolveTarget exige exatamente um entre match_id, espn_match_id, team_id e league e
// traduz o ID da ESPN para o nosso match_id
func resolveTarget(w http.ResponseWriter, r *http.Request, matchID, espnID, teamID int64, league string) (subscriptionTarget, bool) {
	league = strings.ToUpper(strings.TrimSpace(league))
	informados := 0
	for _, id := range []int64{matchID, espnID, teamID} {
		if id > 0 {
			informados++
		}
	}
	if league != "" {
		informados++
	}
	if informados != 1 {
		utils.WriteError(w, http.StatusBadRequest, "Informe um (e só um) entre match_id, espn_match_id, team_id e league")
		return subscriptionTarget{}, false
	}

	switch {
	case league != "":
		if len(league) > 10 {
			utils.WriteError(w, http.StatusNotFound, "Liga não encontrada")
			return subscriptionTarget{}, false
		}
		return subscriptionTarget{kind: "league", league: league}, true
	case teamID > 0:
		return subscriptionTarget{kind: "team", id: teamID}, true
	}

	if espnID > 0 {
		id, err := database.ResolveMatchIDByESPN(r.Context(), espnID)
		if errors.Is(err, database.ErrNotFound) {
			utils.WriteError(w, http.StatusNotFound, "Jogo não encontrado")
			return subscriptionTarget{}, false
		}
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar o jogo")
			return subscriptionTarget{}, false
		}
		matchID = id
	}

	// matches.id é INTEGER: um número maior nem existe na tabela
	if matchID > math.MaxInt32 {
		utils.WriteError(w, http.StatusNotFound, "Jogo não encontrado")
		return subscriptionTarget{}, false
	}
	return subscriptionTarget{kind: "match", id: matchID}, true
}

// AdminPushTestHandler manda uma notificação de teste para um aparelho:
// POST {"device_id": "..."}. Serve para conferir a configuração do Firebase/EAS sem
// esperar um gol.
func AdminPushTestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Use POST")
		return
	}
	var body struct {
		DeviceID string `json:"device_id"`
	}
	if err := decodeBody(r, &body); err != nil || body.DeviceID == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe o device_id do aparelho")
		return
	}

	target, err := database.GetPushTarget(r.Context(), body.DeviceID)
	if errors.Is(err, database.ErrNotFound) {
		utils.WriteError(w, http.StatusNotFound, "Aparelho não registrado para push")
		return
	}
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar o aparelho")
		return
	}

	ticket, err := services.SendTestPush(r.Context(), target)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, "O serviço de push não aceitou o envio: "+err.Error())
		return
	}
	if ticket.Details.Error == "DeviceNotRegistered" {
		database.DeletePushToken(r.Context(), target.Token)
	}
	resp := map[string]string{
		"platform":     target.Platform,
		"expo_status":  ticket.Status,
		"expo_error":   ticket.Details.Error,
		"expo_message": ticket.Message,
	}
	// O aparelho em que o admin testou o push passa a receber os alertas do servidor
	// (ex: banco enchendo). O app registra o push com o token de convidado, então este é
	// o único momento em que o servidor sabe qual aparelho é do admin
	if ticket.Status == "ok" {
		if err := database.MarkAdminAlertDevice(r.Context(), body.DeviceID); err != nil {
			utils.CustomLog("PUSH", "Erro ao ligar os alertas de admin no aparelho: %v", err)
		} else {
			resp["admin_alerts"] = "ativado"
		}
	}
	utils.WriteJSON(w, http.StatusOK, resp)
}

// AdminDBSizeHandler mostra o tamanho do banco e as maiores tabelas: GET /admin/db-size
func AdminDBSizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Use GET")
		return
	}
	report, err := services.GetDBSizeReport(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao medir o banco")
		return
	}
	utils.WriteJSON(w, http.StatusOK, report)
}

// parseKinds valida os tipos de aviso pedidos no POST /push/subscriptions. Omitido (ou
// null): mantém os da inscrição (todos, se for nova). A lista com todos os tipos vira nil
// = todos, inclusive os que surgirem depois. Devolve a mensagem de erro, se houver.
func parseKinds(raw *[]string) (kinds []string, set bool, msg string) {
	if raw == nil {
		return nil, false, ""
	}
	if len(*raw) == 0 {
		return nil, false, "Escolha ao menos um tipo de aviso (para não receber nada, deixe de seguir)"
	}
	valid := map[string]bool{}
	for _, k := range database.PushKinds {
		valid[k] = true
	}
	asked := map[string]bool{}
	for _, k := range *raw {
		if !valid[k] {
			return nil, false, "Tipo de aviso inválido: " + k + " (válidos: " + strings.Join(database.PushKinds, ", ") + ")"
		}
		asked[k] = true
	}
	for _, k := range database.PushKinds { // na ordem do jogo, sem repetidos
		if asked[k] {
			kinds = append(kinds, k)
		}
	}
	if len(kinds) == len(database.PushKinds) {
		return nil, true, ""
	}
	return kinds, true, ""
}

// webSubscription é a inscrição do navegador, como o PushSubscription.toJSON() devolve
type webSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Serviços de push dos navegadores (Chrome/Edge antigo, Firefox, Safari, Edge). Só esses:
// senão alguém poderia fazer o servidor mandar dados para qualquer endereço
var webPushHosts = []string{"fcm.googleapis.com", "push.services.mozilla.com", "web.push.apple.com", "notify.windows.com"}

func registerWebPush(w http.ResponseWriter, r *http.Request, device string, sub *webSubscription) {
	if !services.WebPushEnabled() {
		utils.WriteError(w, http.StatusServiceUnavailable, "Notificações no navegador ainda não estão configuradas no servidor")
		return
	}
	if msg := validateWebSubscription(sub); msg != "" {
		utils.WriteError(w, http.StatusBadRequest, msg)
		return
	}
	if err := database.RegisterWebPushDevice(r.Context(), device, sub.Endpoint, sub.Keys.P256dh, sub.Keys.Auth); err != nil {
		utils.CustomLog("PUSH", "Erro ao registrar navegador: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao registrar o navegador")
		return
	}
	linkDeviceToAccount(r, device)
	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// validateWebSubscription devolve o motivo da recusa ("" = válida)
func validateWebSubscription(sub *webSubscription) string {
	if sub == nil {
		return "Informe subscription (o PushSubscription do navegador, com endpoint e keys)"
	}
	u, err := url.Parse(sub.Endpoint)
	if err != nil || u.Scheme != "https" || len(sub.Endpoint) > 1000 {
		return "subscription.endpoint inválido"
	}
	host := strings.ToLower(u.Hostname())
	known := false
	for _, h := range webPushHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			known = true
			break
		}
	}
	if !known {
		return "subscription.endpoint não é de um serviço de push conhecido"
	}
	if n := base64URLLen(sub.Keys.P256dh); n != 65 {
		return "subscription.keys.p256dh inválida"
	}
	if n := base64URLLen(sub.Keys.Auth); n != 16 {
		return "subscription.keys.auth inválida"
	}
	return ""
}

// base64URLLen devolve o tamanho decodificado da chave (-1 se não for base64url)
func base64URLLen(s string) int {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	if err != nil {
		return -1
	}
	return len(b)
}

// PushWebKeyHandler devolve a chave pública que o site usa para inscrever o navegador:
// GET /push/web-key → {"public_key": "..."} (503 se o push no navegador estiver desligado)
func PushWebKeyHandler(w http.ResponseWriter, r *http.Request) {
	key := services.WebPushPublicKey()
	if key == "" {
		utils.WriteError(w, http.StatusServiceUnavailable, "Notificações no navegador ainda não estão configuradas no servidor")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]string{"public_key": key})
}
