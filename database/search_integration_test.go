package database

import (
	"context"
	"testing"
)

// Busca de jogadores sem acento, pelo índice de trigramas (migration 029)
func TestSearchPlayersGlobalSemAcento(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	limpa := func() { mustExec(t, `DELETE FROM espn_players WHERE espn_id IN (9994001, 9994002)`) }
	limpa()
	t.Cleanup(limpa)
	mustExec(t, `INSERT INTO espn_players (espn_id, name, espn_team_id) VALUES
                 (9994001, 'Éverton Ribeiro', $1), (9994002, 'João Pedro', $1)`, tstESPNHome)

	for busca, esperado := range map[string]string{"everton": "Éverton Ribeiro", "JOAO PED": "João Pedro", "ribeiro": "Éverton Ribeiro"} {
		players, err := SearchPlayersGlobal(ctx, busca)
		if err != nil {
			t.Fatal(err)
		}
		achou := false
		for _, p := range players {
			if p.Name == esperado {
				achou = true
				if p.TeamName != "Casa FC" {
					t.Errorf("%q: time esperado Casa FC, veio %q", busca, p.TeamName)
				}
			}
		}
		if !achou {
			t.Errorf("%q: esperado achar %q, veio %+v", busca, esperado, players)
		}
	}
}
