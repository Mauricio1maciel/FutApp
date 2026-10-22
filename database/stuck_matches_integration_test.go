package database

import (
	"context"
	"testing"
	"time"
)

// Jogos que já deviam ter acabado e continuam marcados ou em andamento
func TestGetStuckMatches(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	preso := newTestMatch(t, 7773001, -5*time.Hour, "TIMED", 0, 0)
	newTestMatch(t, 7773002, -2*time.Hour, "IN_PLAY", 1, 0)   // pode estar rolando ainda
	newTestMatch(t, 7773003, -5*time.Hour, "FINISHED", 2, 1)  // tem resultado
	newTestMatch(t, 7773004, -96*time.Hour, "TIMED", 0, 0)    // antigo demais: status velho conhecido
	newTestMatch(t, 7773005, -5*time.Hour, "POSTPONED", 0, 0) // adiado não é sem resultado
	semESPN := newTestMatch(t, 7773006, -30*time.Hour, "IN_PLAY", 1, 1)

	// A ESPN já tem o fim (0 x 1), mas o resultado não chegou ao jogo: grava sem o
	// trigger, que levaria o placar para o jogo
	conn, err := DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	semTrigger := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	semTrigger(`SET session_replication_role = replica`)
	semTrigger(`INSERT INTO espn_matches (espn_match_id, league, season, match_date, status,
                                          espn_home_team_id, espn_away_team_id, home_score, away_score)
                SELECT 9993001, league, '2026', match_date, 'post', $2, $3, '0', '1'
                FROM matches WHERE id = $1`, preso, tstESPNHome, tstESPNAway)
	semTrigger(`RESET session_replication_role`)

	stuck, err := GetStuckMatches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got []StuckMatch
	for _, s := range stuck {
		if s.League == tstLeague {
			got = append(got, s)
		}
	}
	if len(got) != 2 || got[0].MatchID != semESPN || got[1].MatchID != preso {
		t.Fatalf("esperado o jogo sem a ESPN e o preso (nessa ordem), veio %+v", got)
	}
	if got[0].ESPNStatus != "" || got[0].Status != "IN_PLAY" || got[0].HomeTeam != "Casa FC" {
		t.Errorf("jogo sem a ESPN: %+v", got[0])
	}
	var kickoff time.Time
	DB.QueryRow(`SELECT match_date AT TIME ZONE 'UTC' FROM matches WHERE id = $1`, preso).Scan(&kickoff)
	dia := kickoff.In(time.FixedZone("BRT", -3*60*60)).Format("20060102")
	if g := got[1]; g.ESPNStatus != "post" || g.ESPNHome != "0" || g.ESPNAway != "1" || g.ESPNDay != dia || !g.MatchDate.Equal(kickoff) {
		t.Errorf("jogo preso: esperado ESPN encerrado 0 x 1 no dia %s às %v, veio %+v", dia, kickoff, g)
	}
}
