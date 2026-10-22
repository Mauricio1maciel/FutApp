package services

import (
	"App-Futebol/database"
	"App-Futebol/testutil"
	"context"
	"strings"
	"testing"
)

// Jogo sem resultado ponta a ponta: busca o dia de novo na ESPN (falsa), avisa o admin
// (Expo falso) uma vez por jogo e não avisa o que a nova busca resolveu
func TestCheckStuckMatches(t *testing.T) {
	database.DB = testutil.OpenDB(t, "alert_state")
	ctx := context.Background()
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	limpa := func() {
		exec(`DELETE FROM matches WHERE league = 'TSX'`)
		exec(`DELETE FROM teams WHERE api_id IN (998001, 998002)`)
		exec(`DELETE FROM leagues WHERE code_api = 'TSX'`)
		exec(`DELETE FROM push_devices WHERE device_id = 'alerta-jogos'`)
		exec(`DELETE FROM alert_state WHERE starts_with(key, $1)`, stuckAlertPrefix)
	}
	limpa()
	t.Cleanup(limpa)
	exec(`UPDATE push_devices SET admin_alerts = false`) // banco de teste: só este aparelho recebe
	exec(`INSERT INTO push_devices (device_id, push_token, platform, admin_alerts)
          VALUES ('alerta-jogos', 'ExponentPushToken[alerta-jogos]', 'android', true)`)
	exec(`INSERT INTO leagues (code_api, name) VALUES ('TSX', 'Liga sem resultado de teste')`)
	exec(`INSERT INTO teams (api_id, name) VALUES (998001, 'Casa SR'), (998002, 'Fora SR')`)
	novoJogo := func(idEvent int64, horasAtras int) {
		exec(`INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id,
                                   home_score, away_score, match_date, status)
              VALUES ($1, 'TSX', '2026', 1, 998001, 998002, 0, 0,
                      (now() AT TIME ZONE 'UTC') - $2 * INTERVAL '1 hour', 'TIMED')`, idEvent, horasAtras)
	}
	novoJogo(7782001, 5)

	_, lotes := fakeExpo(t, func(ExpoMessage) ExpoTicket { return ExpoTicket{Status: "ok", ID: "x"} })
	var buscas []string
	resolve := false
	original := resyncESPNDay
	t.Cleanup(func() { resyncESPNDay = original })
	resyncESPNDay = func(ctx context.Context, league, day string) {
		buscas = append(buscas, league+"|"+day)
		if resolve { // a ESPN trouxe o resultado: o trigger encerraria o jogo
			exec(`UPDATE matches SET status = 'FINISHED' WHERE league = 'TSX'`)
		}
	}
	avisos := func() []ExpoMessage {
		var all []ExpoMessage
		for _, l := range *lotes {
			all = append(all, l...)
		}
		return all
	}

	if err := CheckStuckMatches(ctx); err != nil {
		t.Fatal(err)
	}
	if len(buscas) != 1 || !strings.HasPrefix(buscas[0], "TSX|") {
		t.Errorf("esperada 1 nova busca na ESPN, veio %v", buscas)
	}
	if a := avisos(); len(a) != 1 || a[0].Title != "⚠️ Jogo sem resultado" ||
		!strings.HasPrefix(a[0].Body, "Casa SR x Fora SR (TSX, ") || !strings.Contains(a[0].Body, "a ESPN não tem o jogo") ||
		a[0].Data["alert"] != "stuck_matches" {
		t.Fatalf("aviso ao admin: %+v", a)
	}

	// Mesmo jogo na volta seguinte: busca de novo, mas não repete o aviso
	if err := CheckStuckMatches(ctx); err != nil {
		t.Fatal(err)
	}
	if len(buscas) != 2 || len(avisos()) != 1 {
		t.Errorf("esperado buscar de novo sem repetir o aviso: %d buscas, %d avisos", len(buscas), len(avisos()))
	}

	// Outro jogo preso: só ele é avisado
	novoJogo(7782002, 26)
	if err := CheckStuckMatches(ctx); err != nil {
		t.Fatal(err)
	}
	if a := avisos(); len(a) != 2 || a[1].Title != "⚠️ Jogo sem resultado" {
		t.Errorf("o jogo novo deveria ser avisado sozinho: %+v", a)
	}

	// A nova busca resolve: nada a avisar
	exec(`DELETE FROM alert_state WHERE starts_with(key, $1)`, stuckAlertPrefix)
	resolve = true
	if err := CheckStuckMatches(ctx); err != nil {
		t.Fatal(err)
	}
	if len(avisos()) != 2 {
		t.Errorf("a ESPN resolveu: não deveria avisar, veio %d avisos", len(avisos()))
	}
}

func TestStuckAlertTextVariosJogos(t *testing.T) {
	ms := []database.StuckMatch{
		{HomeTeam: "Palmeiras", AwayTeam: "Fluminense", League: "CLI", ESPNStatus: "post", ESPNHome: "0", ESPNAway: "1"},
		{HomeTeam: "Flamengo", AwayTeam: "Estudiantes", League: "CLI", ESPNStatus: "in"},
	}
	title, body := stuckAlertText(ms)
	if title != "⚠️ 2 jogos sem resultado" || !strings.Contains(body, "na ESPN acabou 0 x 1, mas o resultado não chegou ao jogo. E mais 1.") {
		t.Errorf("veio %q | %q", title, body)
	}
}
