package services

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCommentaryEntries(t *testing.T) {
	const raw = `[
	  {"sequence": 0, "time": {"displayValue": ""}, "text": "Início do primeiro tempo."},
	  {"sequence": 40, "time": {"displayValue": "40'"}, "text": " Falta cometida por Caíque Gonçalves (Vitória). ",
	   "play": {"type": {"type": "foul"}, "team": {"displayName": "Vitória"}}},
	  {"sequence": 41, "time": {"displayValue": "41'"}, "text": "   "}
	]`
	var c []espnCommentary
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatal(err)
	}
	got := commentaryEntries(c)
	if len(got) != 2 {
		t.Fatalf("lance sem texto fica de fora: esperado 2, veio %+v", got)
	}
	if e := got[1]; e.Seq != 40 || e.Minute != "40'" || e.Text != "Falta cometida por Caíque Gonçalves (Vitória)." || e.Type != "foul" || e.Team != "Vitória" {
		t.Errorf("lance: %+v", e)
	}
}

func TestRecentForCommentary(t *testing.T) {
	agora := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	if !recentForCommentary("2026-10-19T22:00Z", agora) {
		t.Error("jogo de 5 dias atrás deveria ter narração")
	}
	if recentForCommentary("2026-09-01T22:00Z", agora) || recentForCommentary("", agora) {
		t.Error("jogo de mais de 30 dias (ou sem data) não busca narração")
	}
}
