package services

import (
	"App-Futebol/models"
	"encoding/json"
	"testing"
	"time"
)

// Relógio do /matches/live: segundos de jogo, período, se está andando e quando foi lido
func TestLiveMatchesFromScoreboardRelogio(t *testing.T) {
	const raw = `{"events": [
	  {"id": "1", "competitions": [{"status": {"clock": 1200.0, "displayClock": "20'", "period": 1,
	    "type": {"name": "STATUS_FIRST_HALF", "state": "in"}}, "competitors": [
	    {"homeAway": "home", "score": "1", "team": {"id": "10", "displayName": "Athletic"}},
	    {"homeAway": "away", "score": "0", "team": {"id": "20", "displayName": "Operário"}}]}]},
	  {"id": "2", "competitions": [{"status": {"clock": 2700.0, "displayClock": "45'", "period": 1,
	    "type": {"name": "STATUS_HALFTIME", "state": "in"}}, "competitors": []}]},
	  {"id": "3", "competitions": [{"status": {"clock": 7200.0, "displayClock": "120'", "period": 5,
	    "type": {"name": "STATUS_SHOOTOUT", "state": "in"}}, "competitors": [
	    {"homeAway": "home", "score": "0", "shootoutScore": 2.0, "team": {"id": "30", "displayName": "Flamengo"}},
	    {"homeAway": "away", "score": "0", "shootoutScore": 3.0, "team": {"id": "40", "displayName": "Estudiantes"}}]}]},
	  {"id": "4", "competitions": [{"status": {"clock": 0.0, "displayClock": "0'", "period": 0,
	    "type": {"name": "STATUS_SCHEDULED", "state": "pre"}}, "competitors": []}]}
	]}`
	var data models.ESPNScoreboard
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	lido := time.Date(2026, 10, 23, 23, 10, 5, 0, time.FixedZone("BRT", -3*3600))
	jogos := liveMatchesFromScoreboard(data, lido)
	if len(jogos) != 4 {
		t.Fatalf("esperado 4 jogos, veio %d", len(jogos))
	}

	rolando := jogos[0]
	if rolando.ClockSeconds != 1200 || rolando.Period != 1 || !rolando.ClockRunning || rolando.Clock != "20'" {
		t.Errorf("1º tempo aos 20': %+v", rolando)
	}
	if rolando.ClockAt != "2026-10-24T02:10:05Z" {
		t.Errorf("a hora da leitura vai em UTC: veio %q", rolando.ClockAt)
	}
	if rolando.HomePenalty != nil || rolando.AwayPenalty != nil {
		t.Errorf("sem disputa de pênaltis, os campos ficam vazios: %v %v", rolando.HomePenalty, rolando.AwayPenalty)
	}
	if jogos[1].ClockRunning || jogos[1].ClockSeconds != 2700 {
		t.Errorf("no intervalo o relógio fica parado: %+v", jogos[1])
	}
	penaltis := jogos[2]
	if penaltis.ClockRunning || penaltis.Period != 5 || penaltis.HomePenalty == nil || *penaltis.HomePenalty != 2 || *penaltis.AwayPenalty != 3 {
		t.Errorf("disputa de pênaltis: relógio parado e parcial 2 x 3, veio %+v", penaltis)
	}
	if jogos[3].ClockRunning || jogos[3].Clock != "pre" {
		t.Errorf("jogo que não começou: relógio parado, veio %+v", jogos[3])
	}
}
