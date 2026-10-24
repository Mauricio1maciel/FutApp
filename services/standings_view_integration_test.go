package services

import (
	"App-Futebol/database"
	"App-Futebol/testutil"
	"context"
	"testing"
)

// Tabela como mandante, como visitante e por turno, com 3 times (turnos de 2 rodadas):
//
//	rodada 1: A 2 x 0 B   rodada 2: B 1 x 1 C   rodada 3: C 0 x 1 A   rodada 4: B 3 x 0 A
func TestViewStandings(t *testing.T) {
	database.DB = testutil.OpenDB(t, "matches")
	ctx := context.Background()
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	limpa := func() {
		exec(`DELETE FROM matches WHERE league = 'TSH'`)
		exec(`DELETE FROM teams WHERE api_id IN (999101, 999102, 999103)`)
		exec(`DELETE FROM leagues WHERE code_api = 'TSH'`)
		tableRulesCache.Flush()
	}
	limpa()
	t.Cleanup(limpa)
	exec(`INSERT INTO leagues (code_api, name) VALUES ('TSH', 'Liga dos recortes')`)
	exec(`INSERT INTO teams (api_id, name, crest_url) VALUES (999101, 'A', 'a.png'), (999102, 'B', 'b.png'), (999103, 'C', 'c.png')`)
	exec(`INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id, home_score, away_score, match_date, status) VALUES
          (7786001, 'TSH', '2099', 1, 999101, 999102, 2, 0, '2099-03-01 20:00', 'FINISHED'),
          (7786002, 'TSH', '2099', 2, 999102, 999103, 1, 1, '2099-03-08 20:00', 'FINISHED'),
          (7786003, 'TSH', '2099', 3, 999103, 999101, 0, 1, '2099-03-15 20:00', 'FINISHED'),
          (7786004, 'TSH', '2099', 4, 999102, 999101, 3, 0, '2099-03-22 20:00', 'FINISHED')`)

	ordem := func(view string) (string, map[string]int) {
		t.Helper()
		s, err := ViewStandings(ctx, "TSH", "2099", view)
		if err != nil {
			t.Fatal(err)
		}
		nomes, pontos := "", map[string]int{}
		for _, st := range s {
			nomes += st.TeamName
			pontos[st.TeamName] = st.Points
			if st.Zone != "" || st.CrestURL == "" {
				t.Errorf("%s: recorte sem zona e com escudo, veio %+v", view, st)
			}
		}
		return nomes, pontos
	}

	// Em casa: A venceu B; B empatou com C e venceu A; C perdeu para A
	if o, p := ordem("home"); o != "BAC" || p["B"] != 4 || p["A"] != 3 || p["C"] != 0 {
		t.Errorf("como mandante: esperado B 4, A 3, C 0, veio %s %v", o, p)
	}
	// Fora: A venceu C e perdeu para B; C empatou com B; B perdeu para A
	if o, p := ordem("away"); o != "ACB" || p["A"] != 3 || p["C"] != 1 || p["B"] != 0 {
		t.Errorf("como visitante: esperado A 3, C 1, B 0, veio %s %v", o, p)
	}
	// 1º turno (rodadas 1 e 2): A 3; B e C com 1, C à frente no saldo
	if o, _ := ordem("first"); o != "ACB" {
		t.Errorf("1º turno: esperado A, C, B, veio %s", o)
	}
	// 2º turno (rodadas 3 e 4): A e B com 3, B à frente no saldo
	if o, _ := ordem("second"); o != "BAC" {
		t.Errorf("2º turno: esperado B, A, C, veio %s", o)
	}

	// A forma do recorte só tem os jogos dele: em casa, A venceu o único jogo
	s, _ := ViewStandings(ctx, "TSH", "2099", "home")
	for _, st := range s {
		if st.TeamName == "A" && st.Form != "V" {
			t.Errorf("forma de A em casa: esperado V, veio %q", st.Form)
		}
	}
	if _, err := ViewStandings(ctx, "TSH", "2099", "xyz"); err != ErrUnknownView {
		t.Errorf("recorte desconhecido: esperado ErrUnknownView, veio %v", err)
	}
	if _, err := ViewStandings(ctx, "CLI", "2026", "home"); err != ErrNoLeagueTable {
		t.Errorf("copa: esperado ErrNoLeagueTable, veio %v", err)
	}
}
