package services

import (
	"App-Futebol/database"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestPushText(t *testing.T) {
	pen := func(n int) *int { return &n }
	base := database.PendingPush{HomeTeam: "Flamengo", AwayTeam: "Palmeiras"}

	casos := []struct {
		nome        string
		ajuste      func(p *database.PendingPush)
		title, body string
	}{
		{"início", func(p *database.PendingPush) { p.Kind = "start" },
			"Começou! ⚽", "Flamengo x Palmeiras"},
		{"gol do mandante com autor", func(p *database.PendingPush) {
			p.Kind, p.GoalSide, p.HomeScore, p.Scorer, p.Minute = "goal", "home", 1, "Pedro", "23'"
		}, "⚽ GOL! Flamengo", "Flamengo 1 x 0 Palmeiras · Pedro 23'"},
		{"gol contra do visitante", func(p *database.PendingPush) {
			p.Kind, p.GoalSide, p.HomeScore, p.AwayScore = "goal", "away", 1, 1
			p.Scorer, p.Minute, p.OwnGoal = "Léo Pereira", "90'+3'", true
		}, "⚽ GOL! Palmeiras", "Flamengo 1 x 1 Palmeiras · Léo Pereira (contra) 90'+3'"},
		{"dois gols na mesma leitura", func(p *database.PendingPush) {
			p.Kind, p.GoalSide, p.HomeScore, p.AwayScore = "goal", "both", 2, 1
		}, "⚽ GOLS!", "Flamengo 2 x 1 Palmeiras"},
		{"fim nos pênaltis", func(p *database.PendingPush) {
			p.Kind, p.HomeScore, p.AwayScore = "end", 1, 1
			p.HomePenalty, p.AwayPenalty = pen(4), pen(3)
		}, "Fim de jogo", "Flamengo 1 (4) x (3) 1 Palmeiras"},
		{"escalação, no horário de Brasília", func(p *database.PendingPush) {
			p.Kind = "lineup"
			p.MatchDate = time.Date(2026, 10, 15, 0, 30, 0, 0, time.UTC)
		}, "Escalações confirmadas 📋", "Flamengo x Palmeiras · começa às 21:30"},
	}

	for _, c := range casos {
		p := base
		c.ajuste(&p)
		title, body := pushText(p)
		if title != c.title || body != c.body {
			t.Errorf("%s:\n  veio     %q | %q\n  esperado %q | %q", c.nome, title, body, c.title, c.body)
		}
	}
}

func TestPushDataSemVinculoESPN(t *testing.T) {
	data := pushData(database.PendingPush{Kind: "start", MatchID: 42, League: "BSA"})
	if data["match_id"] != "42" || data["league"] != "BSA" || data["type"] != "start" {
		t.Errorf("dados básicos errados: %v", data)
	}
	if _, ok := data["espn_match_id"]; ok {
		t.Errorf("sem vínculo com a ESPN, espn_match_id não deveria ir: %v", data)
	}
}

// fakeExpo responde como o Expo e guarda o que recebeu
func fakeExpo(t *testing.T, ticket func(msg ExpoMessage) ExpoTicket) (*httptest.Server, *[][]ExpoMessage) {
	t.Helper()
	var lotes [][]ExpoMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msgs []ExpoMessage
		if err := json.NewDecoder(r.Body).Decode(&msgs); err != nil {
			t.Errorf("corpo inválido: %v", err)
		}
		lotes = append(lotes, msgs)
		tickets := make([]ExpoTicket, len(msgs))
		for i, m := range msgs {
			tickets[i] = ticket(m)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"data": tickets})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("EXPO_PUSH_URL", srv.URL)
	return srv, &lotes
}

func TestSendExpoEmLotesDe100(t *testing.T) {
	_, lotes := fakeExpo(t, func(m ExpoMessage) ExpoTicket {
		tk := ExpoTicket{Status: "ok"}
		if m.To == "ExponentPushToken[149]" {
			tk = ExpoTicket{Status: "error"}
			tk.Details.Error = "DeviceNotRegistered"
		}
		return tk
	})

	msgs := make([]ExpoMessage, 150)
	for i := range msgs {
		msgs[i] = ExpoMessage{To: "ExponentPushToken[" + strconv.Itoa(i) + "]", Title: "t"}
	}

	tickets, err := SendExpo(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(*lotes) != 2 || len((*lotes)[0]) != 100 || len((*lotes)[1]) != 50 {
		t.Fatalf("esperado lotes de 100 e 50, veio %d lote(s)", len(*lotes))
	}
	if len(tickets) != 150 || tickets[149].Details.Error != "DeviceNotRegistered" || tickets[0].Status != "ok" {
		t.Errorf("tickets fora de ordem: %+v ... %+v", tickets[0], tickets[149])
	}
}

func TestSendExpoRecusado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errors":[{"code":"VALIDATION_ERROR","message":"\"to\" must be a string"}]}`))
	}))
	defer srv.Close()
	t.Setenv("EXPO_PUSH_URL", srv.URL)

	if _, err := SendExpo(context.Background(), []ExpoMessage{{To: "x"}}); err == nil {
		t.Fatal("esperado erro quando o Expo recusa o envio")
	}
}
