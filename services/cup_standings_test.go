package services

import (
	"App-Futebol/models"
	"testing"
)

func groupMatch(group string, home, away int64, homeScore, awayScore int) models.Match {
	m := finished(home, away, homeScore, awayScore)
	m.Stage = "GROUP_STAGE"
	m.GroupName = group
	return m
}

func TestBuildCupStandingsLibertadores(t *testing.T) {
	// Grupo A: 1 > 2 > 3 > 4. Os jogos de preliminar e de mata-mata não podem contar.
	matches := []models.Match{
		groupMatch("GROUP_A", 1, 2, 1, 0),
		groupMatch("GROUP_A", 1, 3, 1, 0),
		groupMatch("GROUP_A", 1, 4, 1, 0),
		groupMatch("GROUP_A", 2, 3, 1, 0),
		groupMatch("GROUP_A", 2, 4, 1, 0),
		groupMatch("GROUP_A", 3, 4, 1, 0),
	}
	preliminar := finished(4, 5, 9, 0)
	preliminar.Stage = "ROUND_1"
	mataMata := finished(3, 1, 9, 0)
	mataMata.Stage = "QUARTER_FINALS"
	matches = append(matches, preliminar, mataMata)

	s := BuildCupStandings(matches, nil, LibertadoresFormat)

	if len(s) != 4 {
		t.Fatalf("esperado 4 times (só a fase de grupos), veio %d", len(s))
	}
	esperado := []struct {
		id   int64
		zone string
	}{
		{1, "Classificado - Oitavas"},
		{2, "Classificado - Oitavas"},
		{3, "Sul-Americana"},
		{4, "Eliminado"},
	}
	for i, e := range esperado {
		if s[i].TeamID != e.id || s[i].Zone != e.zone || s[i].GroupName != "GROUP_A" {
			t.Errorf("posição %d: esperado time %d em %q, veio time %d em %q (%s)",
				i+1, e.id, e.zone, s[i].TeamID, s[i].Zone, s[i].GroupName)
		}
	}
	if s[0].Played != 3 {
		t.Errorf("líder deveria ter 3 jogos, veio %d", s[0].Played)
	}
}
