package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/testutil"
	"context"
	"testing"
)

// Os jogos encerrados nas últimas 12 horas são buscados de novo: os melhores momentos saem
// depois do apito
func TestRefreshRecentMatchMedia(t *testing.T) {
	database.DB = testutil.OpenDB(t, "espn_matches")
	ctx := context.Background()
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	limpa := func() { exec(`DELETE FROM espn_matches WHERE espn_match_id BETWEEN 9998001 AND 9998009`) }
	limpa()
	t.Cleanup(limpa)
	// Recente e encerrado; antigo; e um em andamento (o placar ao vivo cuida dele)
	exec(`INSERT INTO espn_matches (espn_match_id, league, season, match_date, status) VALUES
          (9998001, 'BSA', '2026', (now() AT TIME ZONE 'UTC') - INTERVAL '3 hours', 'post'),
          (9998002, 'BSA', '2026', (now() AT TIME ZONE 'UTC') - INTERVAL '2 days', 'post'),
          (9998003, 'BSA', '2026', (now() AT TIME ZONE 'UTC') - INTERVAL '1 hour', 'in')`)

	var buscados []string
	original := mediaFetcher
	t.Cleanup(func() { mediaFetcher = original })
	mediaFetcher = func(espnLeague, matchID string) (MatchMedia, error) {
		buscados = append(buscados, espnLeague+"/"+matchID)
		return MatchMedia{Videos: []models.MatchVideo{{ID: "1", Title: "Melhores momentos", HLSURL: "x.m3u8", Highlights: true}}}, nil
	}

	n, err := RefreshRecentMatchMedia(ctx)
	if err != nil || n != 1 {
		t.Fatalf("esperado 1 jogo com vídeo novo, veio %d (%v)", n, err)
	}
	if len(buscados) != 1 || buscados[0] != "bra.1/9998001" {
		t.Errorf("só o jogo encerrado há menos de 12 horas, pelo nome da liga na ESPN: %v", buscados)
	}
	videos, found, err := database.GetMatchVideos(ctx, 9998001)
	if err != nil || !found || len(videos) != 1 || !videos[0].Highlights || videos[0].HLSURL != "x.m3u8" {
		t.Errorf("vídeo gravado: %+v (%v, %v)", videos, found, err)
	}
	// Os mesmos vídeos na volta seguinte: nada muda, nada é regravado
	if n, err := RefreshRecentMatchMedia(ctx); err != nil || n != 0 {
		t.Errorf("sem vídeo novo: esperado 0, veio %d (%v)", n, err)
	}
}
