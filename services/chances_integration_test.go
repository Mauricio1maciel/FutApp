package services

import (
	"App-Futebol/database"
	"App-Futebol/testutil"
	"context"
	"testing"
)

// As chances leem os jogos e as regras da liga do banco (testutil.OpenDB explica como rodar)
func TestLeagueChancesForComOBanco(t *testing.T) {
	database.DB = testutil.OpenDB(t, "matches")
	ctx := context.Background()
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	limpa := func() {
		exec(`DELETE FROM matches WHERE league = 'TSC'`)
		exec(`DELETE FROM teams WHERE api_id IN (997001, 997002)`)
		exec(`DELETE FROM leagues WHERE code_api = 'TSC'`)
	}
	limpa()
	t.Cleanup(limpa)
	exec(`INSERT INTO leagues (code_api, name) VALUES ('TSC', 'Liga de chances de teste')`)
	exec(`INSERT INTO teams (api_id, name, crest_url) VALUES (997001, 'Casa CH', 'casa.png'), (997002, 'Fora CH', 'fora.png')`)
	exec(`INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id, home_score, away_score, match_date, status) VALUES
          (7781001, 'TSC', '2099', 1, 997001, 997002, 2, 0, '2099-03-01 20:00', 'FINISHED'),
          (7781002, 'TSC', '2099', 2, 997002, 997001, 0, 0, '2099-03-08 20:00', 'TIMED')`)

	c, err := LeagueChancesFor(ctx, "TSC", "2099")
	if err != nil {
		t.Fatal(err)
	}
	if c.RemainingMatches != 1 || len(c.Teams) != 2 {
		t.Fatalf("esperado 2 times e 1 jogo por jogar, veio %+v", c)
	}
	lider := c.Teams[0]
	if lider.TeamID != 997001 || lider.Points != 3 || lider.CrestURL != "casa.png" {
		t.Errorf("1º deveria ser Casa CH (3 pontos, com escudo), veio %+v", lider)
	}
	// Casa CH só perde o título se perder por 3 ou mais (ou por 2, no sorteio de desempate)
	if lider.Title < 80 || lider.Title+c.Teams[1].Title < 99.9 || lider.Title+c.Teams[1].Title > 100.1 {
		t.Errorf("título: Casa %.1f%%, Fora %.1f%%", lider.Title, c.Teams[1].Title)
	}
}
