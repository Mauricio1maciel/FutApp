package database

import (
	"App-Futebol/models"
	"context"
	"testing"
	"time"
)

// Ligas que vêm só da ESPN (leagues.espn_only): o jogo é criado a partir da ESPN e os
// times que a football-data não conhece são cadastrados. Roda contra um Postgres de
// verdade (testutil.OpenDB explica como).
func TestLigaSoDaESPNCriaJogoETimes(t *testing.T) {
	openTestDB(t)
	var temColuna bool
	DB.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
                 WHERE table_name = 'leagues' AND column_name = 'espn_only')`).Scan(&temColuna)
	if !temColuna {
		t.Skip("banco de teste sem a migration 012")
	}

	const novoESPN = int64(882999)
	novoAPI := ESPNOnlyTeamIDOffset + novoESPN
	limpa := func() {
		mustExec(t, `DELETE FROM matches WHERE league = 'TSE'`)
		mustExec(t, `DELETE FROM espn_matches WHERE league IN ('TSE', 'TST')`)
		mustExec(t, `DELETE FROM team_leagues WHERE league = 'TSE'`)
		mustExec(t, `DELETE FROM teams WHERE api_id = $1`, novoAPI)
		mustExec(t, `DELETE FROM leagues WHERE code_api = 'TSE'`)
	}
	limpa()
	t.Cleanup(limpa)
	mustExec(t, `INSERT INTO leagues (code_api, name, espn_only) VALUES ('TSE', 'Copa de Teste', true)`)

	ctx := context.Background()
	jogo := models.ESPNMatchDB{
		MatchID: "9990031", League: "TSE", Season: "2026", Status: "pre",
		MatchDate: time.Now().UTC().Add(48 * time.Hour).Format("2006-01-02T15:04Z"),
		Stage:     "GROUP_STAGE", GroupName: "Group A",
		ESPNHomeTeamID: tstESPNHome, HomeTeam: "Casa", // já existe (como os brasileiros)
		ESPNAwayTeamID: novoESPN, AwayTeam: "Visitante ESPN", AwayLogo: "https://logo",
	}
	if err := SaveFullMatchHistory(ctx, jogo, nil, nil); err != nil {
		t.Fatal(err)
	}

	var home, away int64
	var stage, group, status string
	if err := DB.QueryRow(`SELECT api_home_team_id, api_away_team_id, stage, group_name, status
                           FROM matches WHERE league = 'TSE' AND id_event = 9990031`).
		Scan(&home, &away, &stage, &group, &status); err != nil {
		t.Fatalf("o jogo da liga só da ESPN não foi criado: %v", err)
	}
	if home != tstHome || away != novoAPI {
		t.Errorf("times do jogo: esperado %d x %d, veio %d x %d", tstHome, novoAPI, home, away)
	}
	if stage != "GROUP_STAGE" || group != "Group A" || status != "TIMED" {
		t.Errorf("fase/grupo/status: veio %q %q %q", stage, group, status)
	}

	var nome string
	DB.QueryRow(`SELECT name FROM teams WHERE api_id = $1`, novoAPI).Scan(&nome)
	if nome != "Visitante ESPN" {
		t.Errorf("time novo deveria ser cadastrado com a faixa da ESPN, veio %q", nome)
	}
	var casas, vinculos int
	DB.QueryRow(`SELECT count(*) FROM teams WHERE espn_team_id = $1`, tstESPNHome).Scan(&casas)
	DB.QueryRow(`SELECT count(*) FROM team_leagues WHERE league = 'TSE' AND season = '2026'
                 AND team_api_id IN ($1, $2)`, tstHome, novoAPI).Scan(&vinculos)
	if casas != 1 {
		t.Errorf("o time que já existia foi duplicado: %d registros", casas)
	}
	if vinculos != 2 {
		t.Errorf("os dois times deveriam estar vinculados à liga, veio %d", vinculos)
	}

	// Jogo de volta decidido nos pênaltis: o mandante venceu a partida (2 x 1), mas o
	// agregado empatou e o visitante avançou nos pênaltis. O vencedor é quem avançou,
	// como na football-data
	quatro, tres := 4, 3
	jogo.Status, jogo.HomeScore, jogo.AwayScore = "post", "2", "1"
	jogo.HomePenalty, jogo.AwayPenalty = &tres, &quatro
	if err := SaveFullMatchHistory(ctx, jogo, nil, nil); err != nil {
		t.Fatal(err)
	}
	var penCasa, penFora int
	var vencedor string
	if err := DB.QueryRow(`SELECT home_penalty, away_penalty, winner FROM matches
                           WHERE league = 'TSE' AND id_event = 9990031`).Scan(&penCasa, &penFora, &vencedor); err != nil {
		t.Fatalf("pênaltis não chegaram ao jogo: %v", err)
	}
	if penCasa != 3 || penFora != 4 || vencedor != "AWAY_TEAM" {
		t.Errorf("esperado pênaltis 3 x 4 e vencedor AWAY_TEAM, veio %d x %d e %q", penCasa, penFora, vencedor)
	}

	// Liga comum (TST): a ESPN não cria jogo nem cadastra time
	jogo.MatchID, jogo.League, jogo.ESPNAwayTeamID = "9990032", tstLeague, novoESPN+1
	jogo.HomePenalty, jogo.AwayPenalty = nil, nil
	if err := SaveFullMatchHistory(ctx, jogo, nil, nil); err != nil {
		t.Fatal(err)
	}
	var jogosComuns, timesComuns int
	DB.QueryRow(`SELECT count(*) FROM matches WHERE id_event = 9990032`).Scan(&jogosComuns)
	DB.QueryRow(`SELECT count(*) FROM teams WHERE espn_team_id = $1`, novoESPN+1).Scan(&timesComuns)
	if jogosComuns != 0 || timesComuns != 0 {
		t.Errorf("liga comum não deveria criar jogo (%d) nem time (%d) pela ESPN", jogosComuns, timesComuns)
	}
}
