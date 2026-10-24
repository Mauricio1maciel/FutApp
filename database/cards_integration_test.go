package database

import (
	"App-Futebol/models"
	"context"
	"testing"
	"time"
)

// Os cartões do time, com o time do lance vindo da escalação quando a ESPN não informa, e o
// próximo jogo
func TestGetTeamCards(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	jogo := newTestMatch(t, 7775001, -72*time.Hour, "FINISHED", 1, 0)
	semLances := newTestMatch(t, 7775002, -24*time.Hour, "FINISHED", 0, 0)
	proximo := newTestMatch(t, 7775003, 48*time.Hour, "TIMED", 0, 0)
	newTestMatch(t, 7775004, 96*time.Hour, "TIMED", 0, 0)

	var data string
	DB.QueryRow(`SELECT to_char(match_date, 'YYYY-MM-DD"T"HH24:MI"Z"') FROM matches WHERE id = $1`, jogo).Scan(&data)
	espn := models.ESPNMatchDB{MatchID: "9996001", League: tstLeague, Season: "2026", Status: "post", MatchDate: data,
		ESPNHomeTeamID: tstESPNHome, ESPNAwayTeamID: tstESPNAway, HomeScore: "1", AwayScore: "0"}
	escalacao := []models.ESPNLineupDB{
		{MatchID: "9996001", ESPNTeamID: tstESPNHome, ESPNPlayerID: 501, PlayerName: "Volante Casa", IsStarter: true},
		{MatchID: "9996001", ESPNTeamID: tstESPNAway, ESPNPlayerID: 502, PlayerName: "Zagueiro Fora", IsStarter: true},
	}
	eventos := []models.ESPNEventDB{
		{MatchID: "9996001", Minute: "10'", EventType: "Yellow Card", ESPNTeamID: tstESPNHome, PlayerName: "Volante Casa"},
		{MatchID: "9996001", Minute: "20'", EventType: "Yellow Card", ESPNTeamID: tstESPNAway, PlayerName: "Zagueiro Fora"},
		{MatchID: "9996001", Minute: "30'", EventType: "Goal", ESPNTeamID: tstESPNHome, PlayerName: "Atacante Casa"},
		{MatchID: "9996001", Minute: "80'", EventType: "Red Card", PlayerName: "Volante Casa"}, // sem time
	}
	if err := SaveFullMatchHistory(ctx, espn, escalacao, eventos); err != nil {
		t.Fatal(err)
	}

	matches, next, err := GetTeamCards(ctx, tstLeague, "2026", tstHome)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 || matches[0].MatchID != jogo || matches[1].MatchID != semLances {
		t.Fatalf("esperado os 2 jogos encerrados, em ordem, veio %+v", matches)
	}
	if !matches[0].HasData || matches[1].HasData {
		t.Errorf("só o primeiro jogo tem os lances da ESPN: %+v", matches)
	}
	if got := matches[0].Cards; len(got) != 2 || got[0] != (CardEvent{"Volante Casa", 501, "Yellow Card"}) || got[1] != (CardEvent{"Volante Casa", 501, "Red Card"}) {
		t.Errorf("cartões do time (sem o do adversário; o vermelho sem time pela escalação): %+v", got)
	}
	if next == nil || next.MatchID != proximo || !next.Home || next.Opponent != "Fora FC" {
		t.Errorf("próximo jogo: esperado %d em casa contra Fora FC, veio %+v", proximo, next)
	}
	mustExec(t, `DELETE FROM espn_matches WHERE espn_match_id = 9996001`)
}
