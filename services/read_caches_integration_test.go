package services

import (
	"App-Futebol/database"
	"App-Futebol/testutil"
	"context"
	"testing"
)

// As regras da tabela e a fase atual ficam guardadas por alguns minutos nas rotas de
// leitura; erro (ex: banco fora) não fica guardado
func TestCachesDasRotasDeLeitura(t *testing.T) {
	database.DB = testutil.OpenDB(t, "matches")
	ctx := context.Background()
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	limpa := func() {
		exec(`DELETE FROM competition_rules WHERE league = 'TSR'`)
		exec(`DELETE FROM matches WHERE league = 'TSR'`)
		exec(`DELETE FROM leagues WHERE code_api = 'TSR'`)
		tableRulesCache.Flush()
		currentStageCache.Flush()
	}
	limpa()
	t.Cleanup(limpa)
	exec(`INSERT INTO leagues (code_api, name) VALUES ('TSR', 'Liga do cache de teste')`)
	exec(`INSERT INTO competition_rules (league, season, libertadores, pre_libertadores, sul_americana, rebaixamento)
          VALUES ('TSR', '2099', 4, 2, 6, 4)`)

	// Banco "fora" (contexto cancelado): erro, e nada fica guardado
	cancelado, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := cachedTableRules(cancelado, "TSR", "2099"); err == nil {
		t.Fatal("com o banco fora, as regras deveriam dar erro")
	}
	r, err := cachedTableRules(ctx, "TSR", "2099")
	if err != nil || r.rule == nil || r.rule.Libertadores != 4 {
		t.Fatalf("depois do erro, as regras deveriam vir do banco: %+v %v", r.rule, err)
	}
	// Mudou no banco: a rota de leitura segue com a guardada; o worker lê na hora
	exec(`UPDATE competition_rules SET libertadores = 5 WHERE league = 'TSR'`)
	if r, _ := cachedTableRules(ctx, "TSR", "2099"); r.rule.Libertadores != 4 {
		t.Errorf("dentro dos 2 minutos, deveria vir a regra guardada (4), veio %d", r.rule.Libertadores)
	}
	if r, err := loadTableRules(ctx, "TSR", "2099"); err != nil || r.rule.Libertadores != 5 {
		t.Errorf("a leitura na hora deveria trazer a regra nova (5), veio %+v %v", r.rule, err)
	}

	// Liga sem regra cadastrada não é erro
	if r, err := loadTableRules(ctx, "TSR", "1990"); err != nil || r.rule != nil {
		t.Errorf("temporada sem regra: esperado sem erro e sem regra, veio %+v %v", r.rule, err)
	}

	// Fase atual
	exec(`INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id, match_date, status, stage)
          VALUES (7783001, 'TSR', '2099', 7, 1, 2, '2099-05-01 20:00', 'TIMED', 'REGULAR_SEASON')`)
	if _, _, err := CurrentStage(cancelado, "TSR", "2099"); err == nil {
		t.Fatal("com o banco fora, a fase atual deveria dar erro")
	}
	if stage, round, err := CurrentStage(ctx, "TSR", "2099"); err != nil || stage != "REGULAR_SEASON" || round != 7 {
		t.Fatalf("fase atual: esperado REGULAR_SEASON 7, veio %q %d %v", stage, round, err)
	}
	exec(`UPDATE matches SET round = 8 WHERE league = 'TSR'`)
	if _, round, _ := CurrentStage(ctx, "TSR", "2099"); round != 7 {
		t.Errorf("dentro de 1 minuto, deveria vir a rodada guardada (7), veio %d", round)
	}
}
