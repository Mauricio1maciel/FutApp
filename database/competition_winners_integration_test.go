package database

import (
	"context"
	"testing"
)

// Título anotado só para time da liga; campeão de fora tira a vaga; sem mudança, nada muda
func TestSetCompetitionWinner(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	limpa := func() {
		mustExec(t, `DELETE FROM competition_winners WHERE league = $1 AND season = '2099'`, tstLeague)
		mustExec(t, `DELETE FROM team_leagues WHERE league = $1 AND season = '2099'`, tstLeague)
	}
	limpa()
	t.Cleanup(limpa)
	mustExec(t, `INSERT INTO team_leagues (team_api_id, league, season) VALUES ($1, $2, '2099')`, tstHome, tstLeague)

	titulos := func() []string {
		var nomes []string
		rows, err := DB.Query(`SELECT team_name FROM competition_winners
                               WHERE league = $1 AND season = '2099' AND competition = 'copa_brasil'`, tstLeague)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var n string
			rows.Scan(&n)
			nomes = append(nomes, n)
		}
		return nomes
	}

	if err := SetCompetitionWinner(ctx, tstLeague, "2099", "copa_brasil", tstHome); err != nil {
		t.Fatal(err)
	}
	if err := SetCompetitionWinner(ctx, tstLeague, "2099", "copa_brasil", tstHome); err != nil { // de novo: não duplica
		t.Fatal(err)
	}
	if got := titulos(); len(got) != 1 || got[0] != "Casa FC" {
		t.Fatalf("esperado o título do time da liga uma vez só, veio %v", got)
	}

	// Campeão que não está na liga (ex: estrangeiro): a vaga anotada sai
	if err := SetCompetitionWinner(ctx, tstLeague, "2099", "copa_brasil", tstAway); err != nil {
		t.Fatal(err)
	}
	if got := titulos(); len(got) != 0 {
		t.Errorf("campeão de fora da liga não dá vaga: deveria apagar, veio %v", got)
	}
}
