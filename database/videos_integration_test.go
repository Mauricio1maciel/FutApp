package database

import (
	"App-Futebol/models"
	"context"
	"testing"
	"time"
)

func TestVideosGravaLeAtualizaEApaga(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() { mustExec(t, `DELETE FROM espn_matches WHERE espn_match_id IN (9998101, 9998102)`) })
	jogo := models.ESPNMatchDB{MatchID: "9998101", League: tstLeague, Season: "2026", Status: "post",
		MatchDate: time.Now().UTC().Add(-3 * time.Hour).Format("2006-01-02T15:04Z"), HomeScore: "1", AwayScore: "0",
		Videos: []models.MatchVideo{{ID: "1", Title: "Gol", HLSURL: "g.m3u8"}}}
	velho := jogo
	velho.MatchID = "9998102"
	velho.MatchDate = time.Now().UTC().Add(-100 * 24 * time.Hour).Format("2006-01-02T15:04Z")
	for _, m := range []models.ESPNMatchDB{jogo, velho} {
		if err := SaveFullMatchHistory(ctx, m, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Gravação seguinte sem vídeos (a busca em português falhou) mantém os gravados
	semVideos := jogo
	semVideos.Videos = nil
	if err := SaveFullMatchHistory(ctx, semVideos, nil, nil); err != nil {
		t.Fatal(err)
	}
	if v, found, err := GetMatchVideos(ctx, 9998101); err != nil || !found || len(v) != 1 {
		t.Fatalf("esperado o vídeo gravado, veio %+v (%v, %v)", v, found, err)
	}

	novos := []models.MatchVideo{{ID: "2", Title: "Melhores momentos", HLSURL: "m.m3u8", Highlights: true}, {ID: "1", Title: "Gol", HLSURL: "g.m3u8"}}
	if changed, err := UpdateMatchMedia(ctx, 9998101, nil, novos); err != nil || !changed {
		t.Errorf("vídeo novo: esperado gravado, veio %v (%v)", changed, err)
	}
	if changed, _ := UpdateMatchMedia(ctx, 9998101, nil, novos); changed {
		t.Error("os mesmos vídeos não deveriam ser regravados")
	}
	if v, _, _ := GetMatchVideos(ctx, 9998101); len(v) != 2 || v[0].ID != "2" {
		t.Errorf("depois da atualização: %+v", v)
	}

	recentes, err := GetRecentFinishedESPNMatches(ctx, 12*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	achou := false
	for _, m := range recentes {
		if m.ESPNMatchID == 9998102 {
			t.Error("jogo de 100 dias atrás não está entre os recentes")
		}
		if m.ESPNMatchID == 9998101 {
			achou = m.Videos == 2
		}
	}
	if !achou {
		t.Errorf("o jogo de 3 horas atrás (com 2 vídeos) deveria estar entre os recentes: %+v", recentes)
	}

	if _, err := ClearOldVideos(ctx, 90*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if v, _, _ := GetMatchVideos(ctx, 9998102); len(v) != 0 {
		t.Errorf("os vídeos do jogo de 100 dias deveriam ter sido apagados: %+v", v)
	}
	if v, _, _ := GetMatchVideos(ctx, 9998101); len(v) != 2 {
		t.Errorf("os do jogo recente ficam: %+v", v)
	}
}
