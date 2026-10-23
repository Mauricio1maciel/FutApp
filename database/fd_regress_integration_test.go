package database

import (
	"App-Futebol/models"
	"context"
	"database/sql"
	"testing"
	"time"
)

func pint(n int) *int { return &n }

type estadoJogo struct {
	status   string
	casa     int
	fora     int
	penCasa  sql.NullInt64
	penFora  sql.NullInt64
	vencedor string
}

func lerJogo(t *testing.T, idEvent int64) estadoJogo {
	t.Helper()
	var e estadoJogo
	if err := DB.QueryRow(`SELECT status, home_score, away_score, home_penalty, away_penalty, COALESCE(winner, '')
                           FROM matches WHERE league = $1 AND id_event = $2`, tstLeague, idEvent).
		Scan(&e.status, &e.casa, &e.fora, &e.penCasa, &e.penFora, &e.vencedor); err != nil {
		t.Fatal(err)
	}
	return e
}

// A football-data parada (ex: Libertadores 2026) não pode desfazer o resultado que a ESPN
// trouxe; correção, adiamento e mudança de data dela continuam valendo
func TestSaveMatchNaoVoltaParaTras(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	const id = 7771001
	data := "2026-10-22T00:30:00Z"
	salva := func(status string, casa, fora int, pc, pf *int, vencedor, quando string) {
		t.Helper()
		if err := SaveMatch(ctx, id, tstLeague, "2026", 0, tstHome, tstAway, casa, fora, pc, pf,
			quando, status, "SEMI_FINALS", "", vencedor); err != nil {
			t.Fatal(err)
		}
	}

	salva("SCHEDULED", 0, 0, nil, nil, "", data)
	// A ESPN leva o resultado: 0 x 1, pênaltis 3 x 4
	mustExec(t, `UPDATE matches SET status = 'FINISHED', home_score = 0, away_score = 1,
                 home_penalty = 3, away_penalty = 4, winner = 'AWAY_TEAM'
                 WHERE league = $1 AND id_event = $2`, tstLeague, id)

	for _, parada := range []string{"SCHEDULED", "TIMED", "IN_PLAY"} {
		salva(parada, 0, 0, nil, nil, "", data)
		if e := lerJogo(t, id); e.status != "FINISHED" || e.fora != 1 || e.penFora.Int64 != 4 || e.vencedor != "AWAY_TEAM" {
			t.Errorf("football-data parada (%s) desfez o resultado: %+v", parada, e)
		}
	}

	// Correção do resultado final pela football-data: vale
	salva("FINISHED", 1, 1, pint(5), pint(4), "HOME_TEAM", data)
	if e := lerJogo(t, id); e.casa != 1 || e.penCasa.Int64 != 5 || e.vencedor != "HOME_TEAM" {
		t.Errorf("correção da football-data deveria valer: %+v", e)
	}
	// Adiamento: vale
	salva("POSTPONED", 0, 0, nil, nil, "", data)
	if e := lerJogo(t, id); e.status != "POSTPONED" {
		t.Errorf("adiamento deveria valer: %+v", e)
	}
	// Jogo remarcado (outra data): vale, mesmo voltando a "marcado"
	mustExec(t, `UPDATE matches SET status = 'FINISHED' WHERE league = $1 AND id_event = $2`, tstLeague, id)
	salva("SCHEDULED", 0, 0, nil, nil, "", "2026-11-05T00:30:00Z")
	if e := lerJogo(t, id); e.status != "SCHEDULED" {
		t.Errorf("jogo com data nova deveria aceitar a football-data: %+v", e)
	}
}

// Jogo da football-data (parada) que termina nos pênaltis: a ESPN leva placar, pênaltis e
// quem avançou
func TestESPNLevaPenaltisEVencedor(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	inicio := time.Now().UTC().Add(-2 * time.Hour)
	m := newTestMatch(t, 7771101, -2*time.Hour, "SCHEDULED", 0, 0)

	jogo := models.ESPNMatchDB{
		MatchID: "9991101", League: tstLeague, Season: "2026", Status: "post",
		MatchDate:      inicio.Format("2006-01-02T15:04Z"),
		ESPNHomeTeamID: tstESPNHome, ESPNAwayTeamID: tstESPNAway, HomeScore: "0", AwayScore: "1",
		HomePenalty: pint(3), AwayPenalty: pint(4),
	}
	if err := SaveFullMatchHistory(ctx, jogo, nil, nil); err != nil {
		t.Fatal(err)
	}
	e := lerJogo(t, 7771101)
	if e.status != "FINISHED" || e.fora != 1 || e.penCasa.Int64 != 3 || e.penFora.Int64 != 4 || e.vencedor != "AWAY_TEAM" {
		t.Fatalf("a ESPN deveria levar placar, pênaltis e vencedor: %+v (jogo %d)", e, m)
	}

	var quando string
	DB.QueryRow(`SELECT to_char(match_date, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM matches WHERE id = $1`, m).Scan(&quando)
	if err := SaveMatch(ctx, 7771101, tstLeague, "2026", 1, tstHome, tstAway, 0, 0, nil, nil,
		quando, "SCHEDULED", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if e := lerJogo(t, 7771101); e.status != "FINISHED" || e.vencedor != "AWAY_TEAM" {
		t.Errorf("a volta seguinte da football-data parada não pode desfazer: %+v", e)
	}
}

// Ao vivo, com a ESPN acompanhando o jogo, o placar atrasado da football-data não entra
func TestSaveMatchAoVivoFicaComOPlacarDaESPN(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	const id = 7771201
	inicio := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Minute)
	salva := func(status string, casa, fora int) {
		t.Helper()
		if err := SaveMatch(ctx, id, tstLeague, "2026", 1, tstHome, tstAway, casa, fora, nil, nil,
			inicio.Format("2006-01-02T15:04:05Z"), status, "REGULAR_SEASON", "", ""); err != nil {
			t.Fatal(err)
		}
	}

	// Sem a ESPN acompanhando, a football-data é a única fonte: a correção dela vale
	salva("IN_PLAY", 1, 0)
	salva("IN_PLAY", 0, 0)
	if e := lerJogo(t, id); e.casa != 0 {
		t.Fatalf("sem a ESPN, a correção da football-data deveria valer: %+v", e)
	}

	jogo := models.ESPNMatchDB{
		MatchID: "9991201", League: tstLeague, Season: "2026", Status: "in",
		MatchDate:      inicio.Format("2006-01-02T15:04Z"),
		ESPNHomeTeamID: tstESPNHome, ESPNAwayTeamID: tstESPNAway, HomeScore: "1", AwayScore: "0",
	}
	if err := SaveFullMatchHistory(ctx, jogo, nil, nil); err != nil {
		t.Fatal(err)
	}
	salva("IN_PLAY", 0, 0) // a football-data ainda não tem o gol
	salva("PAUSED", 0, 0)
	if e := lerJogo(t, id); e.status != "IN_PLAY" || e.casa != 1 {
		t.Errorf("o placar atrasado da football-data passou por cima do da ESPN: %+v", e)
	}

	// O resultado final da football-data continua valendo
	salva("FINISHED", 2, 0)
	if e := lerJogo(t, id); e.status != "FINISHED" || e.casa != 2 {
		t.Errorf("o resultado final da football-data deveria valer: %+v", e)
	}
}

// Durante a disputa de pênaltis a ESPN manda o parcial (0 x 0 no começo): o placar final
// dela precisa substituir o parcial (Flamengo x Estudiantes 2026 ficou preso em 0 x 0)
func TestESPNPenaltisParciaisSaoSubstituidos(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	inicio := time.Now().UTC().Add(-3 * time.Hour)
	newTestMatch(t, 7771301, -3*time.Hour, "SCHEDULED", 0, 0)
	jogo := models.ESPNMatchDB{
		MatchID: "9991301", League: tstLeague, Season: "2026", Status: "in",
		MatchDate:      inicio.Format("2006-01-02T15:04Z"),
		ESPNHomeTeamID: tstESPNHome, ESPNAwayTeamID: tstESPNAway, HomeScore: "0", AwayScore: "0",
		HomePenalty: pint(0), AwayPenalty: pint(0),
	}
	if err := SaveFullMatchHistory(ctx, jogo, nil, nil); err != nil {
		t.Fatal(err)
	}
	jogo.Status, jogo.HomePenalty, jogo.AwayPenalty = "post", pint(2), pint(3)
	if err := SaveFullMatchHistory(ctx, jogo, nil, nil); err != nil {
		t.Fatal(err)
	}
	if e := lerJogo(t, 7771301); e.status != "FINISHED" || e.penCasa.Int64 != 2 || e.penFora.Int64 != 3 || e.vencedor != "AWAY_TEAM" {
		t.Errorf("esperado encerrado, pênaltis 2 x 3 e vitória do visitante, veio %+v", e)
	}
}
