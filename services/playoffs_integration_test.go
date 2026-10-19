package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/testutil"
	"context"
	"fmt"
	"testing"
	"time"
)

// Liga da ESPN com playoffs depois dos pontos corridos (como a Série B 2026): os jogos além
// do turno e returno não somam na tabela, ganham a fase PLAYOFFS e a rodada 0, e a
// atualização seguinte da ESPN não apaga essa fase (contra um Postgres de verdade)
func TestPlayoffsDepoisDosPontosCorridos(t *testing.T) {
	database.DB = testutil.OpenDB(t, "matches")
	ctx := context.Background()
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	limpa := func() {
		exec(`DELETE FROM matches WHERE league = 'TSP'`)
		exec(`DELETE FROM espn_matches WHERE league = 'TSP'`)
		exec(`DELETE FROM standings WHERE league = 'TSP'`)
		exec(`DELETE FROM team_leagues WHERE league = 'TSP'`)
		exec(`DELETE FROM competition_rules WHERE league = 'TSP'`)
		exec(`DELETE FROM competition_zones WHERE league = 'TSP'`)
		exec(`DELETE FROM teams WHERE api_id BETWEEN 998001 AND 998004`)
		exec(`DELETE FROM leagues WHERE code_api = 'TSP'`)
	}
	limpa()
	t.Cleanup(limpa)

	exec(`INSERT INTO leagues (code_api, name, espn_only) VALUES ('TSP', 'Liga com playoffs', true)`)
	for i := 1; i <= 4; i++ {
		exec(`INSERT INTO teams (api_id, espn_team_id, name) VALUES ($1, $2, $3)`, 998000+i, 888000+i, fmt.Sprintf("Time %d", i))
	}
	// 4 times: 1º sobe direto, 2º e 3º vão aos playoffs, 4º cai
	exec(`INSERT INTO competition_rules (league, season, libertadores, pre_libertadores, sul_americana, rebaixamento)
          VALUES ('TSP', '2099', 1, 2, 0, 1)`)
	exec(`INSERT INTO competition_zones (league, zone_key, zone_name, priority) VALUES
          ('TSP', 'lib', 'acesso', 1), ('TSP', 'pre', 'playoff', 2), ('TSP', 'reb', 'rebaixamento', 3)`)

	inicio := time.Date(2099, 3, 1, 20, 0, 0, 0, time.UTC)
	jogo := func(id, rodada, casa, fora int, status string, gc, gf int, dias int) models.ESPNMatchDB {
		return models.ESPNMatchDB{
			MatchID: fmt.Sprint(id), League: "TSP", Season: "2099", Status: status, Stage: "REGULAR_SEASON",
			MatchDate:      inicio.AddDate(0, 0, dias).Format("2006-01-02T15:04Z"),
			ESPNHomeTeamID: int64(888000 + casa), ESPNAwayTeamID: int64(888000 + fora),
			HomeTeam: fmt.Sprintf("Time %d", casa), AwayTeam: fmt.Sprintf("Time %d", fora),
			HomeScore: fmt.Sprint(gc), AwayScore: fmt.Sprint(gf),
		}
	}
	// Turno e returno: 6 rodadas, uma por semana. O time 1 ganha tudo; 2 e 3 empatam entre si
	turno := [][2]int{{1, 2}, {3, 4}, {1, 3}, {2, 4}, {1, 4}, {2, 3}}
	id := 9998000
	for volta := 0; volta < 2; volta++ {
		for i, par := range turno {
			casa, fora := par[0], par[1]
			if volta == 1 {
				casa, fora = fora, casa
			}
			gc, gf := 1, 0
			switch {
			case fora == 1:
				gc, gf = 0, 2
			case (casa == 2 && fora == 3) || (casa == 3 && fora == 2):
				gc, gf = 1, 1
			case fora == 4 || casa == 4:
				if casa == 4 {
					gc, gf = 0, 1
				}
			}
			id++
			rodada := volta*3 + i/2 + 1
			if err := database.SaveFullMatchHistory(ctx, jogo(id, rodada, casa, fora, "post", gc, gf, 7*(rodada-1)), nil, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Playoff entre o 2º e o 3º, em ida e volta, depois da última rodada
	ida := jogo(9998101, 0, 3, 2, "post", 2, 0, 7*6)
	volta := jogo(9998102, 0, 2, 3, "pre", 0, 0, 7*7)
	for _, j := range []models.ESPNMatchDB{ida, volta} {
		if err := database.SaveFullMatchHistory(ctx, j, nil, nil); err != nil {
			t.Fatal(err)
		}
	}

	if err := RecalculateStandings(ctx, "TSP", "2099"); err != nil {
		t.Fatal(err)
	}

	rows, err := database.DB.Query(`SELECT s.position, s.team_id, s.played, s.zone FROM standings s
                                    WHERE s.league = 'TSP' AND s.season = '2099' ORDER BY s.position`)
	if err != nil {
		t.Fatal(err)
	}
	var zonas []string
	for rows.Next() {
		var pos, played int
		var team int64
		var zone string
		rows.Scan(&pos, &team, &played, &zone)
		if played != 6 {
			t.Errorf("time %d: esperado 6 jogos (os playoffs não somam), veio %d", team, played)
		}
		zonas = append(zonas, zone)
	}
	rows.Close()
	if fmt.Sprint(zonas) != "[acesso playoff playoff rebaixamento]" {
		t.Errorf("zonas: esperado [acesso playoff playoff rebaixamento], veio %v", zonas)
	}

	fase := func(espnID string) (stage string, round int) {
		database.DB.QueryRow(`SELECT COALESCE(stage, ''), COALESCE(round, -1) FROM matches
                              WHERE league = 'TSP' AND id_event = $1::bigint`, espnID).Scan(&stage, &round)
		return
	}
	for _, j := range []models.ESPNMatchDB{ida, volta} {
		if stage, round := fase(j.MatchID); stage != "PLAYOFFS" || round != 0 {
			t.Errorf("playoff %s: esperado fase PLAYOFFS e rodada 0, veio %q e %d", j.MatchID, stage, round)
		}
	}
	if stage, round := fase("9998001"); stage != "REGULAR_SEASON" || round != 1 {
		t.Errorf("jogo da 1ª rodada: veio %q e %d", stage, round)
	}

	// A ESPN atualiza o placar da volta (com a fase da temporada): a fase PLAYOFFS fica
	volta.Status, volta.HomeScore = "in", "1"
	if err := database.SaveFullMatchHistory(ctx, volta, nil, nil); err != nil {
		t.Fatal(err)
	}
	if stage, _ := fase(volta.MatchID); stage != "PLAYOFFS" {
		t.Errorf("a atualização da ESPN não pode apagar a fase PLAYOFFS, veio %q", stage)
	}

	// Na tabela ao vivo, o playoff em andamento também não conta
	live, err := LiveStandings(ctx, "TSP", "2099")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range live {
		if s.Played != 6 || s.Live {
			t.Errorf("tabela ao vivo: time %d com %d jogos (live=%v); o playoff não conta", s.TeamID, s.Played, s.Live)
		}
	}
}
