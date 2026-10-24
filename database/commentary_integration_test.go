package database

import (
	"App-Futebol/models"
	"context"
	"testing"
	"time"
)

func TestNarracaoGravaLeEApaga(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	recente := models.ESPNMatchDB{MatchID: "9997001", League: tstLeague, Season: "2026", Status: "in",
		MatchDate: time.Now().UTC().Add(-time.Hour).Format("2006-01-02T15:04Z"), HomeScore: "0", AwayScore: "0",
		Commentary: []models.CommentaryEntry{
			{Seq: 0, Text: "Início do primeiro tempo."},
			{Seq: 1, Minute: "3'", Text: "Falta cometida por Fulano.", Type: "foul", Team: "Casa FC"},
		}}
	antigo := recente
	antigo.MatchID = "9997002"
	antigo.MatchDate = time.Now().UTC().Add(-40 * 24 * time.Hour).Format("2006-01-02T15:04Z")
	t.Cleanup(func() { mustExec(t, `DELETE FROM espn_matches WHERE espn_match_id IN (9997001, 9997002)`) })
	for _, m := range []models.ESPNMatchDB{recente, antigo} {
		if err := SaveFullMatchHistory(ctx, m, nil, nil); err != nil {
			t.Fatal(err)
		}
	}

	// A gravação seguinte sem narração (a busca em português falhou) mantém a anterior
	semNarracao := recente
	semNarracao.Commentary = nil
	if err := SaveFullMatchHistory(ctx, semNarracao, nil, nil); err != nil {
		t.Fatal(err)
	}
	lances, found, err := GetMatchCommentary(ctx, 9997001)
	if err != nil || !found || len(lances) != 2 {
		t.Fatalf("esperado 2 lances, veio %+v (%v, %v)", lances, found, err)
	}
	if lances[0].Seq != 1 || lances[0].Type != "foul" || lances[1].Seq != 0 {
		t.Errorf("do mais recente para o mais antigo: veio %+v", lances)
	}

	// Limpeza: só a do jogo com mais de 30 dias
	if _, err := ClearOldCommentary(ctx, 30*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if l, _, _ := GetMatchCommentary(ctx, 9997002); len(l) != 0 {
		t.Errorf("a narração do jogo antigo deveria ter sido apagada: %+v", l)
	}
	if l, _, _ := GetMatchCommentary(ctx, 9997001); len(l) != 2 {
		t.Errorf("a do jogo recente fica: %+v", l)
	}
	if l, found, err := GetMatchCommentary(ctx, 1); err != nil || found || l != nil {
		t.Errorf("jogo que não existe: esperado não encontrado, veio %v %v %v", l, found, err)
	}
}
