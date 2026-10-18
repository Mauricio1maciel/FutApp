package services

import (
	"App-Futebol/models"
	"testing"
)

func TestLiveAsFinished(t *testing.T) {
	matches := []models.Match{
		{APIHomeTeamID: 1, APIAwayTeamID: 2, Status: "FINISHED"},
		{APIHomeTeamID: 3, APIAwayTeamID: 4, Status: "IN_PLAY"},
		{APIHomeTeamID: 5, APIAwayTeamID: 6, Status: "PAUSED"}, // intervalo
		{APIHomeTeamID: 7, APIAwayTeamID: 8, Status: "TIMED"},
	}
	out, playing := liveAsFinished(matches)
	if out[1].Status != "FINISHED" || out[2].Status != "FINISHED" || out[3].Status != "TIMED" {
		t.Errorf("status depois: %v %v %v", out[1].Status, out[2].Status, out[3].Status)
	}
	if matches[1].Status != "IN_PLAY" {
		t.Error("a lista original não pode mudar (a forma usa os jogos de verdade)")
	}
	if len(playing) != 4 || !playing[3] || !playing[6] || playing[1] || playing[7] {
		t.Errorf("times jogando: %v", playing)
	}
}

func TestMarkLive(t *testing.T) {
	official := []models.Standing{{TeamID: 1, Position: 1}, {TeamID: 2, Position: 2}, {TeamID: 3, Position: 3}}
	live := []models.Standing{{TeamID: 3, Position: 2}, {TeamID: 1, Position: 1}, {TeamID: 2, Position: 3}}
	markLive(live, official, map[int64]bool{3: true, 2: true}, map[int64]string{3: "escudo3.png"})

	if live[0].TeamID != 1 || live[1].TeamID != 3 || live[2].TeamID != 2 {
		t.Fatalf("ordem por posição errada: %+v", live)
	}
	if live[1].PositionChange != 1 || live[2].PositionChange != -1 || live[0].PositionChange != 0 {
		t.Errorf("variação de posição: %+v", live)
	}
	if !live[1].Live || !live[2].Live || live[0].Live {
		t.Errorf("quem está jogando: %+v", live)
	}
	if live[1].CrestURL != "escudo3.png" {
		t.Errorf("escudo do time 3: %q", live[1].CrestURL)
	}
}
