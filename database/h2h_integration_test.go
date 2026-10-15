package database

import (
	"context"
	"testing"
)

// Confronto direto em qualquer temporada; jogos do time só da temporada atual de cada
// competição (o banco guarda temporadas antigas)
func TestHeadToHeadETemporadaAtual(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	ins := func(idEvent int64, season, date string, home, away int64, hs, as int, status string) {
		mustExec(t, `INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id,
                     home_score, away_score, match_date, status)
                     VALUES ($1, $2, $3, 1, $4, $5, $6, $7, $8::timestamp, $9)`,
			idEvent, tstLeague, season, home, away, hs, as, date, status)
	}
	ins(7770101, "2024", "2024-05-10 20:00:00", tstHome, tstAway, 2, 0, "FINISHED") // temporada antiga
	ins(7770102, "2025", "2025-06-10 20:00:00", tstAway, tstHome, 1, 1, "FINISHED")
	ins(7770103, "2026", "2026-04-10 20:00:00", tstHome, tstAway, 0, 1, "FINISHED")
	ins(7770104, "2026", "2026-11-10 20:00:00", tstAway, tstHome, 0, 0, "TIMED") // ainda vai acontecer

	h2h, err := GetHeadToHead(ctx, tstAway, tstHome) // ordem invertida de propósito
	if err != nil {
		t.Fatal(err)
	}
	if len(h2h) != 3 || h2h[0].Season != "2026" || h2h[2].Season != "2024" {
		t.Fatalf("confronto direto: esperado os 3 jogos encerrados, do mais novo ao mais antigo, veio %d", len(h2h))
	}

	jogos, err := GetMatchesByTeamID(ctx, tstHome, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range jogos {
		if m.Season != "2026" {
			t.Errorf("jogos do time deveriam ser só da temporada atual, veio %s", m.Season)
		}
	}
	if len(jogos) != 2 {
		t.Errorf("esperado 2 jogos de 2026 (um encerrado e um marcado), veio %d", len(jogos))
	}
}
