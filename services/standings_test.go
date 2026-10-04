package services

import (
	"App-Futebol/models"
	"testing"
)

func finished(home, away int64, homeScore, awayScore int) models.Match {
	return models.Match{
		APIHomeTeamID: home, HomeTeam: string(rune('A' + home)),
		APIAwayTeamID: away, AwayTeam: string(rune('A' + away)),
		HomeScore: homeScore, AwayScore: awayScore,
		Status: "FINISHED",
	}
}

func TestBuildStandingsOrdenaPorPontosMesmoSemCriterio(t *testing.T) {
	// Time 1: 6 pts, saldo +2 | Time 2: 3 pts, saldo +4 | Time 3: 0 pts
	matches := []models.Match{
		finished(1, 3, 1, 0),
		finished(1, 2, 1, 0),
		finished(2, 3, 5, 0),
	}

	casos := map[string][]string{
		"sem criterios":           nil,
		"sem points (caso da PL)": {"goal_diff", "goals_for"},
	}

	for nome, criteria := range casos {
		s := BuildStandings(matches, nil, nil, nil, criteria)

		if len(s) != 3 {
			t.Fatalf("%s: esperado 3 times, veio %d", nome, len(s))
		}
		ordem := []int64{s[0].TeamID, s[1].TeamID, s[2].TeamID}
		if ordem[0] != 1 || ordem[1] != 2 || ordem[2] != 3 {
			t.Errorf("%s: ordem esperada [1 2 3], veio %v", nome, ordem)
		}
	}
}

func TestBuildStandingsNaoCriaTimeFantasma(t *testing.T) {
	matches := []models.Match{finished(1, 2, 2, 1)}
	s := BuildStandings(matches, nil, nil, nil, []string{"points"})

	if len(s) != 2 {
		t.Errorf("esperado 2 times, veio %d", len(s))
	}
}
