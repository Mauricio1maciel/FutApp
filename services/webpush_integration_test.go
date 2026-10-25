package services

import (
	"App-Futebol/database"
	"App-Futebol/testutil"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// O mesmo aviso vai para o app (Expo) e para o navegador (Web Push); navegador com
// inscrição expirada é removido (contra um Postgres de verdade: testutil.OpenDB)
func TestAvisoNoAppENoNavegador(t *testing.T) {
	database.DB = testutil.OpenDB(t, "push_outbox")
	ctx := context.Background()
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	limpa := func() {
		exec(`DELETE FROM matches WHERE league = 'TSW'`)
		exec(`DELETE FROM push_devices WHERE device_id LIKE 'teste-web-%'`)
		exec(`DELETE FROM teams WHERE api_id IN (997001, 997002)`)
	}
	limpa()
	t.Cleanup(limpa)
	setVAPID(t)

	exec(`INSERT INTO teams (api_id, name, short) VALUES (997001, 'Casa FC', 'Casa'), (997002, 'Fora FC', 'Fora')`)
	var m int64
	if err := database.DB.QueryRow(`
        INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id,
                             home_score, away_score, match_date, status)
        VALUES (7790001, 'TSW', '2026', 1, 997001, 997002, 0, 0, now() AT TIME ZONE 'UTC', 'IN_PLAY')
        RETURNING id`).Scan(&m); err != nil {
		t.Fatal(err)
	}

	// Serviço de push do navegador falso: responde 201 até "expirar" a inscrição
	var mu sync.Mutex
	var bodies [][]byte
	status := http.StatusCreated
	pushService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		bodies = append(bodies, b)
		w.WriteHeader(status)
	}))
	defer pushService.Close()

	b := newBrowser(t)
	web := b.target(pushService.URL + "/push/teste")
	if err := database.RegisterPushDevice(ctx, "teste-web-app", "ExponentPushToken[web-app]", "android"); err != nil {
		t.Fatal(err)
	}
	if err := database.RegisterWebPushDevice(ctx, "teste-web-site", web.Token, web.P256dh, web.Auth); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"teste-web-app", "teste-web-site"} {
		if err := database.SubscribeMatch(ctx, database.Device(d), m, nil, false); err != nil {
			t.Fatal(err)
		}
	}
	_, lotes := fakeExpo(t, func(ExpoMessage) ExpoTicket { return ExpoTicket{Status: "ok", ID: "x"} })

	exec(`UPDATE matches SET home_score = 1 WHERE id = $1`, m) // gol
	ProcessPushOutbox(ctx)

	if len(*lotes) != 1 || len((*lotes)[0]) != 1 || (*lotes)[0][0].To != "ExponentPushToken[web-app]" {
		t.Errorf("o app deveria receber 1 aviso pelo Expo, veio %+v", *lotes)
	}
	mu.Lock()
	if len(bodies) != 1 {
		t.Fatalf("o navegador deveria receber 1 aviso, veio %d", len(bodies))
	}
	var p webPushPayload
	if err := json.Unmarshal(b.decrypt(t, bodies[0]), &p); err != nil || p.Title != "⚽ GOL! Casa" || p.Data["type"] != "goal" {
		t.Errorf("aviso no navegador: %+v (%v)", p, err)
	}
	status = http.StatusGone // o usuário tirou a permissão no navegador
	mu.Unlock()

	var result string
	database.DB.QueryRow(`SELECT result FROM push_outbox WHERE match_id = $1 ORDER BY id DESC LIMIT 1`, m).Scan(&result)
	if result != "ok 2/2" {
		t.Errorf("resultado na fila: esperado \"ok 2/2\", veio %q", result)
	}

	exec(`UPDATE matches SET home_score = 2 WHERE id = $1`, m) // outro gol
	ProcessPushOutbox(ctx)
	if _, err := database.GetPushTarget(ctx, "teste-web-site"); err != database.ErrNotFound {
		t.Errorf("inscrição expirada (410) deveria ser removida, veio %v", err)
	}
}
