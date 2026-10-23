package database

import (
	"App-Futebol/models"
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func jogoFD(id int64, status string, casa, fora int) MatchRow {
	return MatchRow{IDEvent: id, League: tstLeague, Season: "2026", Round: 1, HomeTeamID: tstHome, AwayTeamID: tstAway,
		HomeScore: casa, AwayScore: fora, Date: "2026-11-01T20:00:00Z", Status: status, Stage: "REGULAR_SEASON"}
}

// A temporada inteira num comando só, sem regravar o que não mudou
func TestSaveMatchesSoGravaOQueMudou(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	jogos := []MatchRow{jogoFD(7774001, "TIMED", 0, 0), jogoFD(7774002, "TIMED", 0, 0), jogoFD(7774003, "TIMED", 0, 0)}

	if n, err := SaveMatches(ctx, jogos); err != nil || n != 3 {
		t.Fatalf("3 jogos novos: esperado 3 gravados, veio %d (%v)", n, err)
	}
	if n, err := SaveMatches(ctx, jogos); err != nil || n != 0 {
		t.Errorf("a mesma temporada de novo: esperado 0 regravados, veio %d (%v)", n, err)
	}
	// Um jogo mudou, e o mesmo jogo veio duas vezes (fica a última versão)
	jogos[1] = jogoFD(7774002, "FINISHED", 2, 1)
	jogos = append(jogos, jogoFD(7774002, "FINISHED", 3, 1))
	if n, err := SaveMatches(ctx, jogos); err != nil || n != 1 {
		t.Errorf("um jogo mudou: esperado 1 gravado, veio %d (%v)", n, err)
	}
	if e := lerJogo(t, 7774002); e.status != "FINISHED" || e.casa != 3 {
		t.Errorf("esperado o último 3 x 1 encerrado, veio %+v", e)
	}

	// O jogo único também não regrava o igual
	if n, err := saveMatch(ctx, jogoFD(7774001, "TIMED", 0, 0)); err != nil || n != 0 {
		t.Errorf("jogo igual: esperado 0, veio %d (%v)", n, err)
	}

	// Um jogo com problema não trava os outros: o comando único falha e grava um por um
	ruim := jogoFD(7774004, "TIMED", 0, 0)
	ruim.Date = "data-invalida"
	n, err := SaveMatches(ctx, []MatchRow{jogoFD(7774005, "TIMED", 0, 0), ruim})
	if err == nil || n != 1 {
		t.Errorf("com um jogo ruim: esperado 1 gravado e o erro do ruim, veio %d (%v)", n, err)
	}
	if e := lerJogo(t, 7774005); e.status != "TIMED" {
		t.Errorf("o jogo bom deveria ter sido gravado: %+v", e)
	}
}

// Escalação e eventos num comando cada, na ordem em que a ESPN mandou
func TestSaveFullMatchHistoryEmLote(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	inicio := time.Now().UTC().Add(-time.Hour)
	jogo := models.ESPNMatchDB{MatchID: "9995001", League: tstLeague, Season: "2026", Status: "in",
		MatchDate: inicio.Format("2006-01-02T15:04Z"), ESPNHomeTeamID: tstESPNHome, ESPNAwayTeamID: tstESPNAway,
		HomeScore: "1", AwayScore: "1"}
	escalacao := []models.ESPNLineupDB{
		{MatchID: "9995001", ESPNTeamID: tstESPNHome, ESPNPlayerID: 1, PlayerName: "Goleiro", Jersey: "1", IsStarter: true, Formation: "4-3-3"},
		{MatchID: "9995001", ESPNTeamID: tstESPNAway, ESPNPlayerID: 2, PlayerName: "Atacante", Jersey: "9", IsStarter: true},
		{MatchID: "9995001", ESPNTeamID: tstESPNAway, ESPNPlayerID: 3, PlayerName: "Reserva", Jersey: "20"},
	}
	eventos := []models.ESPNEventDB{
		{MatchID: "9995001", Minute: "10'", EventType: "Goal", ESPNTeamID: tstESPNHome, PlayerName: "A"},
		{MatchID: "9995001", Minute: "30'", EventType: "Yellow Card", ESPNTeamID: tstESPNAway, PlayerName: "B"},
		{MatchID: "9995001", Minute: "60'", EventType: "Goal", ESPNTeamID: tstESPNAway, PlayerName: "C", Details: "D"},
	}
	for i := 0; i < 2; i++ { // o worker grava a cada minuto: a segunda substitui a primeira
		if err := SaveFullMatchHistory(ctx, jogo, escalacao, eventos); err != nil {
			t.Fatal(err)
		}
	}

	var titulares, reservas int
	DB.QueryRow(`SELECT count(*) FILTER (WHERE is_starter), count(*) FILTER (WHERE NOT is_starter)
                 FROM espn_match_lineups WHERE espn_match_id = 9995001`).Scan(&titulares, &reservas)
	if titulares != 2 || reservas != 1 {
		t.Errorf("escalação: esperado 2 titulares e 1 reserva, veio %d e %d", titulares, reservas)
	}
	rows, err := DB.Query(`SELECT minute || ' ' || player_name || ' ' || COALESCE(details, '')
                           FROM espn_match_events WHERE espn_match_id = 9995001 ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ordem []string
	for rows.Next() {
		var e string
		rows.Scan(&e)
		ordem = append(ordem, strings.TrimSpace(e))
	}
	if want := []string{"10' A", "30' B", "60' C D"}; !reflect.DeepEqual(ordem, want) {
		t.Errorf("eventos fora de ordem ou repetidos: veio %v, esperado %v", ordem, want)
	}
	mustExec(t, `DELETE FROM espn_matches WHERE espn_match_id = 9995001`)
}
