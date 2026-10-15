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

func TestLeagueTableMatchesIgnoraMataMata(t *testing.T) {
	fase := func(stage string, home, away int64) models.Match {
		m := finished(home, away, 1, 0)
		m.Stage = stage
		return m
	}
	matches := []models.Match{
		fase("LEAGUE_STAGE", 1, 2),
		fase("REGULAR_SEASON", 1, 3),
		fase("", 2, 3),               // gravado sem fase
		fase("PLAYOFFS", 1, 2),       // mata-mata da Champions: não conta
		fase("QUARTER_FINALS", 2, 3), // não conta
		fase("FINAL", 1, 3),          // não conta
		{APIHomeTeamID: 1, APIAwayTeamID: 99, HomeTeam: "B", Stage: "LEAGUE_STAGE"}, // time desconhecido
	}

	got := leagueTableMatches("CL", matches)
	if len(got) != 3 {
		t.Fatalf("esperado 3 jogos na tabela, veio %d: %+v", len(got), got)
	}
	for _, m := range got {
		if m.Stage != "" && m.Stage != "REGULAR_SEASON" && m.Stage != "LEAGUE_STAGE" {
			t.Errorf("jogo de mata-mata entrou na tabela: %s", m.Stage)
		}
	}
}

func TestRecentForm(t *testing.T) {
	jogo := func(date string, home, away int64, hs, as int) models.Match {
		m := finished(home, away, hs, as)
		m.DateEvent = date
		return m
	}
	matches := []models.Match{
		jogo("2026-08-07 20:00:00", 1, 2, 0, 1), // 1 perde (fora de ordem de propósito)
		jogo("2026-08-01 20:00:00", 1, 2, 2, 0), // 1 vence
		jogo("2026-08-14 20:00:00", 3, 1, 1, 1), // empate
		jogo("2026-08-21 20:00:00", 1, 3, 3, 0),
		jogo("2026-08-28 20:00:00", 2, 1, 0, 2),
		jogo("2026-09-04 20:00:00", 1, 2, 1, 1),
	}
	aberto := jogo("2026-09-11 20:00:00", 1, 3, 0, 0)
	aberto.Status = "TIMED" // não jogado: não conta
	matches = append(matches, aberto)

	form := recentForm(matches)
	// 6 jogos encerrados do time 1: V D E V V E → os últimos 5, do mais antigo ao mais novo
	if form[1] != "DEVVE" {
		t.Errorf("forma do time 1: esperado DEVVE, veio %q", form[1])
	}
	if form[2] != "DVDE" {
		t.Errorf("forma do time 2: esperado DVDE, veio %q", form[2])
	}
}
