package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/services"
	"App-Futebol/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
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
		utils.WriteError(w, http.StatusBadRequest, "Use o token de convidado (com device_id) nas rotas /push")
		return "", false
	}
	return device, true
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
			Token    string `json:"token"`
			Platform string `json:"platform"`
		}
		if err := decodeBody(r, &body); err != nil {
			utils.WriteError(w, http.StatusBadRequest, "JSON inválido")
			return
		}
		if !isExpoToken(body.Token) {
			utils.WriteError(w, http.StatusBadRequest, "Token inválido: esperado ExponentPushToken[...]")
			return
		}
		if body.Platform != "" && body.Platform != "android" && body.Platform != "ios" {
			utils.WriteError(w, http.StatusBadRequest, "platform deve ser android ou ios")
			return
		}
		if err := database.RegisterPushDevice(r.Context(), device, body.Token, body.Platform); err != nil {
			utils.CustomLog("PUSH", "Erro ao registrar aparelho: %v", err)
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao registrar o aparelho")
			return
		}
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

// PushSubscriptionsHandler: GET lista o que o aparelho segue; POST passa a seguir um
// jogo ou um time; DELETE deixa de seguir
func PushSubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	device, ok := pushDevice(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		matches, teams, err := database.GetPushSubscriptions(r.Context(), device)
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar o que você segue")
			return
		}
		utils.WriteJSON(w, http.StatusOK, map[string][]int64{"matches": matches, "teams": teams})

	case http.MethodPost:
		var body struct {
			MatchID     flexID `json:"match_id"`
			ESPNMatchID flexID `json:"espn_match_id"`
			TeamID      flexID `json:"team_id"`
		}
		if err := decodeBody(r, &body); err != nil {
			utils.WriteError(w, http.StatusBadRequest, "JSON inválido")
			return
		}
		target, ok := resolveTarget(w, r, int64(body.MatchID), int64(body.ESPNMatchID), int64(body.TeamID))
		if !ok {
			return
		}

		var err error
		if target.isTeam {
			err = database.SubscribeTeam(r.Context(), device, target.id)
		} else {
			err = database.SubscribeMatch(r.Context(), device, target.id)
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
			utils.WriteJSON(w, http.StatusOK, target.response())
		}

	case http.MethodDelete:
		q := r.URL.Query()
		matchID, _ := strconv.ParseInt(q.Get("match_id"), 10, 64)
		espnID, _ := strconv.ParseInt(q.Get("espn_match_id"), 10, 64)
		teamID, _ := strconv.ParseInt(q.Get("team_id"), 10, 64)
		target, ok := resolveTarget(w, r, matchID, espnID, teamID)
		if !ok {
			return
		}

		var err error
		if target.isTeam {
			err = database.UnsubscribeTeam(r.Context(), device, target.id)
		} else {
			err = database.UnsubscribeMatch(r.Context(), device, target.id)
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
	id     int64
	isTeam bool
}

func (t subscriptionTarget) label() string {
	if t.isTeam {
		return "Time"
	}
	return "Jogo"
}

// response devolve o ID resolvido: quem segue pelo ID da ESPN fica sabendo o match_id
func (t subscriptionTarget) response() map[string]interface{} {
	if t.isTeam {
		return map[string]interface{}{"status": "ok", "team_id": t.id}
	}
	return map[string]interface{}{"status": "ok", "match_id": t.id}
}

// resolveTarget exige exatamente um entre match_id, espn_match_id e team_id e traduz o
// ID da ESPN para o nosso match_id
func resolveTarget(w http.ResponseWriter, r *http.Request, matchID, espnID, teamID int64) (subscriptionTarget, bool) {
	informados := 0
	for _, id := range []int64{matchID, espnID, teamID} {
		if id > 0 {
			informados++
		}
	}
	if informados != 1 {
		utils.WriteError(w, http.StatusBadRequest, "Informe um (e só um) entre match_id, espn_match_id e team_id")
		return subscriptionTarget{}, false
	}

	if teamID > 0 {
		return subscriptionTarget{id: teamID, isTeam: true}, true
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
	return subscriptionTarget{id: matchID}, true
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

	token, err := database.GetPushToken(r.Context(), body.DeviceID)
	if errors.Is(err, database.ErrNotFound) {
		utils.WriteError(w, http.StatusNotFound, "Aparelho não registrado para push")
		return
	}
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar o aparelho")
		return
	}

	ticket, err := services.SendTestPush(r.Context(), token)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, "O Expo não aceitou o envio: "+err.Error())
		return
	}
	if ticket.Details.Error == "DeviceNotRegistered" {
		database.DeletePushToken(r.Context(), token)
	}
	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"expo_status":  ticket.Status,
		"expo_error":   ticket.Details.Error,
		"expo_message": ticket.Message,
	})
}
