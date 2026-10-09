package services

import (
	"App-Futebol/database"
	"App-Futebol/testutil"
	"context"
	"strconv"
	"testing"
)

// Ponta a ponta do worker: gol no banco → fila → Expo (falso) → fila esvaziada.
// Roda contra um Postgres de verdade (testutil.OpenDB explica como).
func TestProcessPushOutboxPontaAPonta(t *testing.T) {
	database.DB = testutil.OpenDB(t, "push_outbox")
	ctx := context.Background()

	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	limpa := func() {
		exec(`DELETE FROM matches WHERE league = 'TSS'`)
		exec(`DELETE FROM push_devices WHERE device_id LIKE 'teste-svc-%'`)
		exec(`DELETE FROM teams WHERE api_id IN (992001, 992002)`)
	}
	limpa()
	t.Cleanup(limpa)

	exec(`INSERT INTO teams (api_id, name, short) VALUES (992001, 'Casa FC', 'Casa'), (992002, 'Fora FC', 'Fora')`)
	var m int64
	if err := database.DB.QueryRow(`
        INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id,
                             home_score, away_score, match_date, status)
        VALUES (7780001, 'TSS', '2026', 1, 992001, 992002, 0, 0, now() AT TIME ZONE 'UTC', 'TIMED')
        RETURNING id`).Scan(&m); err != nil {
		t.Fatal(err)
	}

	for _, d := range []struct{ device, token string }{
		{"teste-svc-vivo", "ExponentPushToken[vivo]"},
		{"teste-svc-morto", "ExponentPushToken[morto]"}, // app desinstalado
	} {
		if err := database.RegisterPushDevice(ctx, d.device, d.token, "android"); err != nil {
			t.Fatal(err)
		}
		if err := database.SubscribeMatch(ctx, d.device, m); err != nil {
			t.Fatal(err)
		}
	}

	_, lotes := fakeExpo(t, func(msg ExpoMessage) ExpoTicket {
		if msg.To == "ExponentPushToken[morto]" {
			tk := ExpoTicket{Status: "error", Message: "not a registered push notification recipient"}
			tk.Details.Error = "DeviceNotRegistered"
			return tk
		}
		return ExpoTicket{Status: "ok", ID: "ticket"}
	})

	exec(`UPDATE matches SET status = 'IN_PLAY', home_score = 1 WHERE id = $1`, m) // começou e já saiu gol
	ProcessPushOutbox(ctx)

	if len(*lotes) != 2 {
		t.Fatalf("esperado 2 envios ao Expo (início e gol), veio %d", len(*lotes))
	}
	inicio, gol := (*lotes)[0], (*lotes)[1]
	if len(inicio) != 2 || inicio[0].Title != "Começou! ⚽" {
		t.Errorf("início: esperado 2 mensagens \"Começou! ⚽\", veio %+v", inicio)
	}
	if gol[0].Title != "⚽ GOL! Casa" || gol[0].Body != "Casa 1 x 0 Fora" {
		t.Errorf("gol: veio %q | %q", gol[0].Title, gol[0].Body)
	}
	if gol[0].Data["match_id"] != strconv.FormatInt(m, 10) || gol[0].Data["type"] != "goal" {
		t.Errorf("dados para abrir o jogo errados: %v", gol[0].Data)
	}

	var resultados []string
	rows, _ := database.DB.Query(`SELECT result FROM push_outbox WHERE match_id = $1 ORDER BY id`, m)
	for rows.Next() {
		var r string
		rows.Scan(&r)
		resultados = append(resultados, r)
	}
	rows.Close()
	// O aparelho morto sai já no primeiro envio, então o gol vai só para o vivo
	if len(resultados) != 2 || resultados[0] != "ok 1/2" || resultados[1] != "ok 1/1" {
		t.Errorf("resultados na fila: %q", resultados)
	}

	if _, err := database.GetPushToken(ctx, "teste-svc-morto"); err != database.ErrNotFound {
		t.Errorf("o aparelho que o Expo deu como DeviceNotRegistered deveria ter saído: %v", err)
	}

	ProcessPushOutbox(ctx) // nada pendente: não pode reenviar
	if len(*lotes) != 2 {
		t.Errorf("reenviou avisos já enviados: %d envios", len(*lotes))
	}
}
