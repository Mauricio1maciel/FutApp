package services

import (
	"App-Futebol/database"
	"App-Futebol/testutil"
	"context"
	"math"
	"testing"
)

func TestMatchPreviewFor(t *testing.T) {
	database.DB = testutil.OpenDB(t, "matches")
	ctx := context.Background()
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	limpa := func() {
		exec(`DELETE FROM matches WHERE league = 'TSP'`)
		exec(`DELETE FROM teams WHERE api_id IN (999201, 999202, 999203)`)
		exec(`DELETE FROM leagues WHERE code_api = 'TSP'`)
	}
	limpa()
	t.Cleanup(limpa)
	exec(`INSERT INTO leagues (code_api, name) VALUES ('TSP', 'Liga da prévia')`)
	exec(`INSERT INTO teams (api_id, name, crest_url) VALUES (999201, 'Forte', 'f.png'), (999202, 'Fraco', 'r.png'), (999203, 'Outro', 'o.png')`)
	exec(`INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id, home_score, away_score, match_date, status) VALUES
          (7787001, 'TSP', '2099', 1, 999201, 999203, 3, 0, '2099-03-01 20:00', 'FINISHED'),
          (7787002, 'TSP', '2099', 2, 999203, 999202, 2, 0, '2099-03-08 20:00', 'FINISHED'),
          (7787003, 'TSP', '2099', 3, 999202, 999201, 0, 2, '2099-03-15 20:00', 'FINISHED')`)
	var id int64
	if err := database.DB.QueryRow(`INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id, home_score, away_score, match_date, status)
          VALUES (7787004, 'TSP', '2099', 4, 999201, 999202, 0, 0, '2099-03-22 20:00', 'TIMED') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}

	p, err := MatchPreviewFor(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if p.Match.MatchID != id || p.Home.Name != "Forte" || p.Away.Name != "Fraco" || p.Home.CrestURL != "f.png" {
		t.Errorf("jogo e times: %+v / %+v / %+v", p.Match, p.Home, p.Away)
	}
	o := p.Probabilities
	if o.HomeWin <= o.AwayWin || math.Abs(o.HomeWin+o.Draw+o.AwayWin-100) > 0.2 || o.BasedOnMatches != 3 {
		t.Errorf("o mais forte em casa deveria ser favorito, somando 100%%: %+v", o)
	}
	if p.Home.Form != "VV" || p.Away.Form != "DD" {
		t.Errorf("forma: Forte %q, Fraco %q (esperado VV e DD)", p.Home.Form, p.Away.Form)
	}
	if p.H2H.Played != 1 || p.H2H.HomeWins != 1 || len(p.H2H.Last) != 1 {
		t.Errorf("confronto direto (Forte venceu fora): %+v", p.H2H)
	}
	if p.Home.Suspended == nil || p.Home.TopScorers == nil {
		t.Errorf("listas vazias vêm como [], não null: %+v", p.Home)
	}
	if _, err := MatchPreviewFor(ctx, 1<<40); err != database.ErrNotFound {
		t.Errorf("jogo que não existe: esperado ErrNotFound, veio %v", err)
	}
}
