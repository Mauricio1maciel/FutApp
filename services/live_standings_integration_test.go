package services

import (
	"App-Futebol/database"
	"App-Futebol/testutil"
	"context"
	"testing"
)

// Virada ao vivo: Fora perdeu o primeiro jogo e está ganhando o segundo; com o placar de
// agora, passa à frente (contra um Postgres de verdade: testutil.OpenDB explica como)
func TestLiveStandings(t *testing.T) {
	database.DB = testutil.OpenDB(t, "matches")
	ctx := context.Background()
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	limpa := func() {
		exec(`DELETE FROM matches WHERE league = 'TSV'`)
		exec(`DELETE FROM teams WHERE api_id IN (996001, 996002)`)
		exec(`DELETE FROM leagues WHERE code_api = 'TSV'`)
	}
	limpa()
	t.Cleanup(limpa)
	exec(`INSERT INTO leagues (code_api, name) VALUES ('TSV', 'Liga ao vivo de teste')`)
	exec(`INSERT INTO teams (api_id, name, crest_url) VALUES (996001, 'Casa AV', 'casa.png'), (996002, 'Fora AV', 'fora.png')`)
	exec(`INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id, home_score, away_score, match_date, status) VALUES
          (7780001, 'TSV', '2099', 1, 996001, 996002, 1, 0, '2099-03-01 20:00', 'FINISHED'),
          (7780002, 'TSV', '2099', 2, 996002, 996001, 2, 0, '2099-03-08 20:00', 'IN_PLAY')`)

	live, err := LiveStandings(ctx, "TSV", "2099")
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 2 {
		t.Fatalf("esperado 2 times, veio %+v", live)
	}
	primeiro, segundo := live[0], live[1]
	// Ao vivo: os dois com 3 pontos; Fora com saldo +1 passa à frente
	if primeiro.TeamID != 996002 || primeiro.Points != 3 || primeiro.PositionChange != 1 || !primeiro.Live {
		t.Errorf("1º ao vivo deveria ser Fora (3 pts, +1 posição, jogando), veio %+v", primeiro)
	}
	if segundo.TeamID != 996001 || segundo.PositionChange != -1 || !segundo.Live || segundo.CrestURL != "casa.png" {
		t.Errorf("2º ao vivo deveria ser Casa (-1 posição, jogando, com escudo), veio %+v", segundo)
	}
	// A forma só conta o que já acabou de verdade
	if primeiro.Form != "D" || segundo.Form != "V" {
		t.Errorf("forma: Fora %q, Casa %q (esperado D e V)", primeiro.Form, segundo.Form)
	}
}
