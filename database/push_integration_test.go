package database

import (
	"App-Futebol/models"
	"App-Futebol/testutil"
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// Testes de integração das notificações push, contra um Postgres de verdade
// (testutil.OpenDB explica como rodar). Sem TEST_DATABASE_URL, são pulados.

const (
	tstLeague   = "TST"
	tstHome     = int64(991001)
	tstAway     = int64(991002)
	tstESPNHome = int64(881001)
	tstESPNAway = int64(881002)
)

func openTestDB(t *testing.T) {
	t.Helper()
	DB = testutil.OpenDB(t, "push_outbox")
	cleanPushTestData(t)
	t.Cleanup(func() { cleanPushTestData(t) })

	mustExec(t, `INSERT INTO teams (api_id, espn_team_id, name, short) VALUES
        ($1, $3, 'Casa FC', 'Casa'), ($2, $4, 'Fora FC', 'Fora')`,
		tstHome, tstAway, tstESPNHome, tstESPNAway)
}

func cleanPushTestData(t *testing.T) {
	mustExec(t, `DELETE FROM matches WHERE league = $1`, tstLeague) // cascata: fila, estado, inscrições
	mustExec(t, `DELETE FROM espn_matches WHERE league = $1`, tstLeague)
	mustExec(t, `DELETE FROM push_devices WHERE device_id LIKE 'teste-push-%'`)
	mustExec(t, `DELETE FROM teams WHERE api_id IN ($1, $2)`, tstHome, tstAway)
}

func mustExec(t *testing.T, query string, args ...interface{}) {
	t.Helper()
	if _, err := DB.Exec(query, args...); err != nil {
		t.Fatalf("%v\n%s", err, query)
	}
}

// newTestMatch cria um jogo entre os times de teste, começando daqui a `kickoff`
func newTestMatch(t *testing.T, idEvent int64, kickoff time.Duration, status string, home, away int) int64 {
	t.Helper()
	var id int64
	err := DB.QueryRow(`
        INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id,
                             home_score, away_score, match_date, status)
        VALUES ($1, $2, '2026', 1, $3, $4, $5, $6, (now() AT TIME ZONE 'UTC') + $7 * INTERVAL '1 second', $8)
        RETURNING id`,
		idEvent, tstLeague, tstHome, tstAway, home, away, kickoff.Seconds(), status).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func newTestDevice(t *testing.T, device, token string) {
	t.Helper()
	if err := RegisterPushDevice(context.Background(), device, token, "android"); err != nil {
		t.Fatal(err)
	}
}

func setScore(t *testing.T, matchID int64, status string, home, away int) {
	t.Helper()
	mustExec(t, `UPDATE matches SET status = $2, home_score = $3, away_score = $4 WHERE id = $1`,
		matchID, status, home, away)
}

// outbox devolve a fila do jogo como texto, ex: "goal home 1-0"
func outbox(t *testing.T, matchID int64) []string {
	t.Helper()
	rows, err := DB.Query(`
        SELECT kind, goal_side, home_score, away_score FROM push_outbox
        WHERE match_id = $1 ORDER BY id`, matchID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := []string{}
	for rows.Next() {
		var kind, side string
		var h, a int
		rows.Scan(&kind, &side, &h, &a)
		if side != "" {
			kind += " " + side
		}
		got = append(got, fmt.Sprintf("%s %d-%d", kind, h, a))
	}
	return got
}

func assertOutbox(t *testing.T, matchID int64, want ...string) {
	t.Helper()
	if want == nil {
		want = []string{}
	}
	if got := outbox(t, matchID); !reflect.DeepEqual(got, want) {
		t.Errorf("fila de avisos:\n  veio     %q\n  esperado %q", got, want)
	}
}

func TestPushJogoCompleto(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	m := newTestMatch(t, 7770001, 0, "TIMED", 0, 0)
	newTestDevice(t, "teste-push-1", "ExponentPushToken[t1]")
	if err := SubscribeMatch(ctx, "teste-push-1", m); err != nil {
		t.Fatal(err)
	}

	setScore(t, m, "IN_PLAY", 0, 0)  // começou
	setScore(t, m, "IN_PLAY", 1, 0)  // gol da casa
	setScore(t, m, "IN_PLAY", 0, 0)  // football-data regrava placar atrasado: nada
	setScore(t, m, "IN_PLAY", 1, 0)  // ESPN corrige: o mesmo gol, nada
	setScore(t, m, "PAUSED", 1, 0)   // intervalo: nada
	setScore(t, m, "IN_PLAY", 1, 1)  // gol de fora
	setScore(t, m, "FINISHED", 2, 1) // gol no fim, junto com o apito
	setScore(t, m, "IN_PLAY", 2, 1)  // fonte atrasada volta para ao vivo: nada
	setScore(t, m, "FINISHED", 2, 1) // fim de novo: nada

	assertOutbox(t, m, "start 0-0", "goal home 1-0", "goal away 1-1", "goal home 2-1", "end 2-1")
}

func TestPushSemSeguidores(t *testing.T) {
	openTestDB(t)
	m := newTestMatch(t, 7770002, 0, "TIMED", 0, 0)
	setScore(t, m, "IN_PLAY", 1, 0)
	assertOutbox(t, m) // ninguém segue: nada na fila
}

func TestPushSeguindoTime(t *testing.T) {
	openTestDB(t)
	m := newTestMatch(t, 7770003, 0, "TIMED", 0, 0)
	newTestDevice(t, "teste-push-1", "ExponentPushToken[t1]")
	if err := SubscribeTeam(context.Background(), "teste-push-1", tstAway); err != nil {
		t.Fatal(err)
	}
	setScore(t, m, "IN_PLAY", 0, 1)
	assertOutbox(t, m, "start 0-1", "goal away 0-1")
}

func TestPushJogoAntigoNaoAvisa(t *testing.T) {
	openTestDB(t)
	m := newTestMatch(t, 7770004, -48*time.Hour, "TIMED", 0, 0)
	newTestDevice(t, "teste-push-1", "ExponentPushToken[t1]")
	SubscribeMatch(context.Background(), "teste-push-1", m)

	setScore(t, m, "FINISHED", 3, 2) // GetMissingMatches completando jogo de 2 dias atrás
	assertOutbox(t, m)
}

func TestPushSoOPlacarFinalQuandoPerdeuOJogo(t *testing.T) {
	openTestDB(t)
	m := newTestMatch(t, 7770012, -2*time.Hour, "TIMED", 0, 0)
	newTestDevice(t, "teste-push-1", "ExponentPushToken[t1]")
	SubscribeMatch(context.Background(), "teste-push-1", m)

	setScore(t, m, "FINISHED", 2, 1) // worker fora do ar durante o jogo
	setScore(t, m, "FINISHED", 3, 1) // correção de placar depois do fim: nada de "gol"
	assertOutbox(t, m, "end 2-1")
}

func TestPushSeguiuNoMeioDoJogo(t *testing.T) {
	openTestDB(t)
	m := newTestMatch(t, 7770005, -30*time.Minute, "IN_PLAY", 1, 0)
	newTestDevice(t, "teste-push-1", "ExponentPushToken[t1]")
	SubscribeMatch(context.Background(), "teste-push-1", m)

	setScore(t, m, "IN_PLAY", 2, 0)
	// Sem "começou" (já tinha começado) e só o gol novo
	assertOutbox(t, m, "goal home 2-0")
}

func TestPushFalhaNoAvisoNaoImpedePlacar(t *testing.T) {
	openTestDB(t)
	m := newTestMatch(t, 7770006, 0, "TIMED", 0, 0)
	newTestDevice(t, "teste-push-1", "ExponentPushToken[t1]")
	SubscribeMatch(context.Background(), "teste-push-1", m)

	// Tudo numa transação desfeita no fim: faz a fila recusar qualquer aviso e confere
	// que o placar continua sendo gravado
	tx, err := DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`ALTER TABLE push_outbox ADD CONSTRAINT teste_recusa_tudo CHECK (false) NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`UPDATE matches SET status = 'IN_PLAY', home_score = 1 WHERE id = $1`, m); err != nil {
		t.Fatalf("o placar não foi gravado por causa do aviso: %v", err)
	}
	var home, avisos int
	tx.QueryRow(`SELECT home_score FROM matches WHERE id = $1`, m).Scan(&home)
	tx.QueryRow(`SELECT count(*) FROM push_outbox WHERE match_id = $1`, m).Scan(&avisos)
	if home != 1 {
		t.Errorf("placar esperado 1, veio %d", home)
	}
	if avisos != 0 {
		t.Errorf("a fila deveria ter recusado os avisos, veio %d", avisos)
	}
}

func TestPushEscalacaoUmaVez(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	m := newTestMatch(t, 7770007, 30*time.Minute, "TIMED", 0, 0)
	newTestDevice(t, "teste-push-1", "ExponentPushToken[t1]")
	SubscribeMatch(ctx, "teste-push-1", m)

	var kickoff time.Time
	DB.QueryRow(`SELECT match_date FROM matches WHERE id = $1`, m).Scan(&kickoff)
	espn := models.ESPNMatchDB{
		MatchID: "9990007", League: tstLeague, Season: "2026", Status: "pre",
		MatchDate:      kickoff.Format("2006-01-02T15:04Z"),
		ESPNHomeTeamID: tstESPNHome, ESPNAwayTeamID: tstESPNAway,
	}
	lineup := []models.ESPNLineupDB{
		{MatchID: "9990007", ESPNTeamID: tstESPNHome, ESPNPlayerID: 1, PlayerName: "Goleiro", IsStarter: true},
		{MatchID: "9990007", ESPNTeamID: tstESPNAway, ESPNPlayerID: 2, PlayerName: "Atacante", IsStarter: true},
	}

	for i := 0; i < 2; i++ { // o worker grava a escalação a cada 5 minutos
		if err := SaveFullMatchHistory(ctx, espn, lineup, nil); err != nil {
			t.Fatal(err)
		}
	}
	assertOutbox(t, m, "lineup 0-0")
}

func TestPushAutorDoGol(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	m := newTestMatch(t, 7770008, -20*time.Minute, "IN_PLAY", 0, 0)
	mustExec(t, `INSERT INTO espn_matches (espn_match_id, league, season, match_date, status,
                     espn_home_team_id, espn_away_team_id)
                 SELECT 9990008, $1, '2026', match_date, 'pre', $2, $3 FROM matches WHERE id = $4`,
		tstLeague, tstESPNHome, tstESPNAway, m)
	mustExec(t, `INSERT INTO espn_match_events (espn_match_id, minute, event_type, espn_team_id, player_name)
                 VALUES (9990008, '23''', 'Goal - Header', $1, 'Pedro')`, tstESPNHome)
	newTestDevice(t, "teste-push-1", "ExponentPushToken[t1]")
	SubscribeMatch(ctx, "teste-push-1", m)

	setScore(t, m, "IN_PLAY", 1, 0) // a ESPN já tem o gol: vai com o autor
	setScore(t, m, "IN_PLAY", 2, 0) // a ESPN ainda não tem o 2º gol: sem autor

	pending, err := GetPendingPushes(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	var gols []PendingPush
	for _, p := range pending {
		if p.MatchID == m && p.Kind == "goal" {
			gols = append(gols, p)
		}
	}
	if len(gols) != 2 {
		t.Fatalf("esperado 2 gols na fila, veio %d", len(gols))
	}
	if gols[0].Scorer != "Pedro" || gols[0].Minute != "23'" || gols[0].ESPNMatchID != 9990008 {
		t.Errorf("1º gol: autor esperado Pedro 23' (ESPN 9990008), veio %q %q (%d)",
			gols[0].Scorer, gols[0].Minute, gols[0].ESPNMatchID)
	}
	if gols[1].Scorer != "" {
		t.Errorf("2º gol: a ESPN só tem 1 gol do time, então o autor (%q) seria o do gol anterior", gols[1].Scorer)
	}
	if gols[0].HomeTeam != "Casa" {
		t.Errorf("nome curto esperado Casa, veio %q", gols[0].HomeTeam)
	}
}

func TestPushInscricoes(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	m := newTestMatch(t, 7770009, time.Hour, "TIMED", 0, 0)

	if err := SubscribeMatch(ctx, "teste-push-sem-registro", m); !errors.Is(err, ErrDeviceNotRegistered) {
		t.Errorf("seguir sem registrar: esperado ErrDeviceNotRegistered, veio %v", err)
	}

	newTestDevice(t, "teste-push-1", "ExponentPushToken[t1]")
	if err := SubscribeMatch(ctx, "teste-push-1", 2147480000); !errors.Is(err, ErrNotFound) {
		t.Errorf("jogo inexistente: esperado ErrNotFound, veio %v", err)
	}
	if err := SubscribeTeam(ctx, "teste-push-1", 2147480000); !errors.Is(err, ErrNotFound) {
		t.Errorf("time inexistente: esperado ErrNotFound, veio %v", err)
	}

	// Segue o jogo e o time mandante: recebe uma vez só
	SubscribeMatch(ctx, "teste-push-1", m)
	SubscribeMatch(ctx, "teste-push-1", m) // repetir não dá erro
	SubscribeTeam(ctx, "teste-push-1", tstHome)
	newTestDevice(t, "teste-push-2", "ExponentPushToken[t2]")
	SubscribeTeam(ctx, "teste-push-2", tstAway)

	tokens, _ := GetPushTokensForMatch(ctx, m)
	if len(tokens) != 2 {
		t.Errorf("esperado 2 aparelhos (sem repetir), veio %v", tokens)
	}

	matches, teams, _ := GetPushSubscriptions(ctx, "teste-push-1")
	if !reflect.DeepEqual(matches, []int64{m}) || !reflect.DeepEqual(teams, []int64{tstHome}) {
		t.Errorf("inscrições: veio jogos %v times %v", matches, teams)
	}

	// App reinstalado: o mesmo token chega com outro device_id; o registro antigo sai
	newTestDevice(t, "teste-push-3", "ExponentPushToken[t1]")
	if _, err := GetPushToken(ctx, "teste-push-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("o device_id antigo deveria ter saído, veio %v", err)
	}
	tokens, _ = GetPushTokensForMatch(ctx, m)
	if !reflect.DeepEqual(tokens, []string{"ExponentPushToken[t2]"}) {
		t.Errorf("as inscrições do aparelho antigo deveriam ter saído junto, veio %v", tokens)
	}
}

func TestPushExpiraAvisoVelho(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	m := newTestMatch(t, 7770010, 0, "TIMED", 0, 0)
	mustExec(t, `INSERT INTO push_outbox (match_id, kind, created_at)
                 VALUES ($1, 'goal', now() - INTERVAL '20 minutes'), ($1, 'end', now())`, m)

	n, err := ExpireStalePushes(ctx, 15*time.Minute)
	if err != nil || n != 1 {
		t.Fatalf("esperado 1 aviso expirado, veio %d (%v)", n, err)
	}
	var pendentes int
	DB.QueryRow(`SELECT count(*) FROM push_outbox WHERE match_id = $1 AND sent_at IS NULL`, m).Scan(&pendentes)
	if pendentes != 1 {
		t.Errorf("o aviso novo deveria continuar na fila, pendentes = %d", pendentes)
	}
}

// O app segue jogos pelo match_id que vem nas listas de jogos
func TestMatchIDNasListasDeJogos(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	m := newTestMatch(t, 7770011, time.Hour, "TIMED", 0, 0)

	porLiga, err := GetMatchesByLeague(ctx, tstLeague, "2026", MatchFilter{})
	if err != nil || len(porLiga) != 1 || porLiga[0].MatchID != m {
		t.Errorf("/matches: esperado match_id %d, veio %+v (%v)", m, porLiga, err)
	}
	porTime, err := GetMatchesByTeamID(ctx, tstHome, nil)
	if err != nil || len(porTime) != 1 || porTime[0].MatchID != m {
		t.Errorf("/team/matches: esperado match_id %d, veio %+v (%v)", m, porTime, err)
	}
	if porLiga[0].IDEvent != "0" {
		t.Errorf("sem vínculo com a ESPN, id_event deveria ser \"0\", veio %q", porLiga[0].IDEvent)
	}
}
