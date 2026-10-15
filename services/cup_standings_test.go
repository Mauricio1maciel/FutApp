package services

import (
	"App-Futebol/models"
	"encoding/json"
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

func TestBuildCupStandingsSulAmericana(t *testing.T) {
	matches := []models.Match{
		groupMatch("Group A", 1, 2, 1, 0),
		groupMatch("Group A", 1, 3, 1, 0),
		groupMatch("Group A", 1, 4, 1, 0),
		groupMatch("Group A", 2, 3, 1, 0),
		groupMatch("Group A", 2, 4, 1, 0),
		groupMatch("Group A", 3, 4, 1, 0),
	}
	s := BuildCupStandings(matches, nil, SulAmericanaFormat)

	esperado := []string{"Classificado - Oitavas", "Playoffs (pré-oitavas)", "Eliminado", "Eliminado"}
	for i, zona := range esperado {
		if s[i].Zone != zona {
			t.Errorf("%dº colocado: esperado %q, veio %q", i+1, zona, s[i].Zone)
		}
	}
}

func TestNormalizeESPNStage(t *testing.T) {
	casos := map[string]string{
		"group-stage":             "GROUP_STAGE",
		"knockout-round-playoffs": "PLAYOFFS",
		"round-of-16":             "LAST_16",
		"quarterfinals":           "QUARTER_FINALS",
		"semifinals":              "SEMI_FINALS",
		"final":                   "FINAL",
		"league-phase":            "league-phase", // fora da lista: fica como veio
		"":                        "",
	}
	for espn, esperado := range casos {
		if got := normalizeESPNStage(espn); got != esperado {
			t.Errorf("%q: esperado %q, veio %q", espn, esperado, got)
		}
	}
}

func TestIsPlaceholderTeam(t *testing.T) {
	for nome, esperado := range map[string]bool{
		"TBD Home": true, "TBD Away": true, "tbd": true,
		"Boca Juniors": false, "": false, "Tbilisi": false,
	} {
		if got := isPlaceholderTeam(nome); got != esperado {
			t.Errorf("%q: esperado %v, veio %v", nome, esperado, got)
		}
	}
}

func TestMatchFromScoreboardEventPenaltis(t *testing.T) {
	// Formato real da ESPN (final da Champions 2025-26, decidida nos pênaltis)
	raw := `{"id": "401862897", "date": "2026-05-30T19:00Z", "season": {"slug": "final"},
	  "competitions": [{"status": {"type": {"state": "post"}}, "competitors": [
	    {"homeAway": "home", "score": "1", "shootoutScore": 4, "team": {"id": "160", "displayName": "Paris Saint-Germain"}},
	    {"homeAway": "away", "score": "1", "shootoutScore": 3, "team": {"id": "359", "displayName": "Arsenal"}}]}]}`
	var ev models.ESPNEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatal(err)
	}
	m := matchFromScoreboardEvent(ev, "CL", "2025-2026")
	if m.HomePenalty == nil || m.AwayPenalty == nil || *m.HomePenalty != 4 || *m.AwayPenalty != 3 {
		t.Fatalf("pênaltis esperados 4 x 3, veio %v x %v", m.HomePenalty, m.AwayPenalty)
	}
	if m.Stage != "FINAL" {
		t.Errorf("fase esperada FINAL, veio %q", m.Stage)
	}

	// Jogo sem disputa de pênaltis: nil, para não gravar 0 x 0 de pênaltis
	ev.Competitions[0].Competitors[0].ShootoutScore = nil
	ev.Competitions[0].Competitors[1].ShootoutScore = nil
	if m := matchFromScoreboardEvent(ev, "CL", "2025-2026"); m.HomePenalty != nil || m.AwayPenalty != nil {
		t.Errorf("sem disputa, pênaltis deveriam ser nil, veio %v x %v", m.HomePenalty, m.AwayPenalty)
	}
}

func TestStatsFromBoxscore(t *testing.T) {
	raw := `{"boxscore": {"teams": [
	  {"homeAway": "home", "statistics": [{"name": "possessionPct", "displayValue": "50.2"}, {"name": "totalShots", "displayValue": "21"}, {"name": "esquisito", "displayValue": "-"}]},
	  {"homeAway": "away", "statistics": [{"name": "possessionPct", "displayValue": "49.8"}, {"name": "totalShots", "displayValue": "10"}]}]}}`
	var data models.ESPNSummaryResponse
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	s := statsFromBoxscore(data)
	if s == nil || s.Home["possessionPct"] != 50.2 || s.Away["totalShots"] != 10 {
		t.Fatalf("leitura errada: %+v", s)
	}
	if _, ok := s.Home["esquisito"]; ok {
		t.Error("valor que não é número deveria ser ignorado")
	}

	if statsFromBoxscore(models.ESPNSummaryResponse{}) != nil {
		t.Error("sem boxscore deveria ser nil")
	}
}
