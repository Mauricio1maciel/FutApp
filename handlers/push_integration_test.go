package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/models"
	"App-Futebol/testutil"
	"App-Futebol/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// Rotas /push/* e /admin/push/test com os middlewares de verdade, contra um Postgres
// de verdade (testutil.OpenDB explica como rodar)
func TestRotasPush(t *testing.T) {
	database.DB = testutil.OpenDB(t, "push_outbox")
	t.Setenv("JWT_SECRET", "segredo-de-teste")
	t.Setenv("ADMIN_KEY", "chave-de-teste")

	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	limpa := func() {
		exec(`DELETE FROM matches WHERE league = 'TSH'`)
		exec(`DELETE FROM espn_matches WHERE league = 'TSH'`)
		exec(`DELETE FROM push_devices WHERE device_id LIKE 'teste-rota-%'`)
		exec(`DELETE FROM teams WHERE api_id IN (993001, 993002)`)
	}
	limpa()
	t.Cleanup(limpa)

	exec(`INSERT INTO teams (api_id, espn_team_id, name) VALUES (993001, 883001, 'Casa FC'), (993002, 883002, 'Fora FC')`)
	var m int64
	if err := database.DB.QueryRow(`
        INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id,
                             home_score, away_score, match_date, status)
        VALUES (7790001, 'TSH', '2026', 1, 993001, 993002, 0, 0,
                (now() AT TIME ZONE 'UTC') + INTERVAL '1 hour', 'TIMED')
        RETURNING id`).Scan(&m); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO espn_matches (espn_match_id, league, season, match_date, status, espn_home_team_id, espn_away_team_id)
          SELECT 9990021, 'TSH', '2026', match_date, 'pre', 883001, 883002 FROM matches WHERE id = $1`, m)

	guest, _ := utils.GenerateToken("teste-rota-1")
	admin, _ := utils.GenerateUserToken(1, models.RoleAdmin)

	register := middlewares.JWTAuth(PushRegisterHandler)
	subs := middlewares.JWTAuth(PushSubscriptionsHandler)
	pushTest := middlewares.AdminAuth(AdminPushTestHandler)

	call := func(h http.HandlerFunc, method, path, token, body string, headers ...string) (int, map[string]interface{}) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		for i := 0; i+1 < len(headers); i += 2 {
			req.Header.Set(headers[i], headers[i+1])
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		var out map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	casos := []struct {
		nome         string
		h            http.HandlerFunc
		method, path string
		token, body  string
		status       int
	}{
		{"seguir antes de registrar", subs, "POST", "/push/subscriptions", guest, fmt.Sprintf(`{"match_id": %d}`, m), 409},
		{"sem token", register, "POST", "/push/register", "", `{}`, 401},
		{"token de push inválido", register, "POST", "/push/register", guest, `{"token": "abc"}`, 400},
		{"token de admin não identifica o aparelho", register, "POST", "/push/register", admin, `{"token": "ExponentPushToken[r1]"}`, 400},
		{"plataforma inválida", register, "POST", "/push/register", guest, `{"token": "ExponentPushToken[r1]", "platform": "windows"}`, 400},
		{"registra o aparelho", register, "POST", "/push/register", guest, `{"token": "ExponentPushToken[r1]", "platform": "android"}`, 200},
		{"segue o jogo", subs, "POST", "/push/subscriptions", guest, fmt.Sprintf(`{"match_id": %d}`, m), 200},
		{"segue o mesmo jogo de novo", subs, "POST", "/push/subscriptions", guest, fmt.Sprintf(`{"match_id": "%d"}`, m), 200},
		{"segue o time", subs, "POST", "/push/subscriptions", guest, `{"team_id": 993001}`, 200},
		{"nenhum ID", subs, "POST", "/push/subscriptions", guest, `{}`, 400},
		{"dois IDs", subs, "POST", "/push/subscriptions", guest, fmt.Sprintf(`{"match_id": %d, "team_id": 993001}`, m), 400},
		{"ID que não é número", subs, "POST", "/push/subscriptions", guest, `{"match_id": "abc"}`, 400},
		{"jogo inexistente", subs, "POST", "/push/subscriptions", guest, `{"match_id": 2147480000}`, 404},
		{"ID maior que um INTEGER", subs, "POST", "/push/subscriptions", guest, `{"match_id": 99999999999}`, 404},
		{"time inexistente", subs, "POST", "/push/subscriptions", guest, `{"team_id": 2147480000}`, 404},
		{"jogo da ESPN inexistente", subs, "POST", "/push/subscriptions", guest, `{"espn_match_id": 1}`, 404},
		{"método errado", subs, "PUT", "/push/subscriptions", guest, ``, 405},
		{"convidado na rota de admin", pushTest, "POST", "/admin/push/test", guest, `{"device_id": "teste-rota-1"}`, 403},
	}
	for _, c := range casos {
		if status, out := call(c.h, c.method, c.path, c.token, c.body); status != c.status {
			t.Errorf("%s: esperado %d, veio %d %v", c.nome, c.status, status, out)
		}
	}

	// Seguir pelo ID da ESPN (texto, como o app guarda) devolve o nosso match_id
	exec(`DELETE FROM match_subscriptions WHERE device_id = 'teste-rota-1'`)
	status, out := call(subs, "POST", "/push/subscriptions", guest, `{"espn_match_id": "9990021"}`)
	if status != 200 || out["match_id"] != float64(m) {
		t.Errorf("seguir pelo ID da ESPN: esperado 200 com match_id %d, veio %d %v", m, status, out)
	}

	status, out = call(subs, "GET", "/push/subscriptions", guest, ``)
	if status != 200 || !reflect.DeepEqual(out["matches"], []interface{}{float64(m)}) ||
		!reflect.DeepEqual(out["teams"], []interface{}{float64(993001)}) {
		t.Errorf("lista do que segue: veio %d %v", status, out)
	}

	if status, _ := call(subs, "DELETE", fmt.Sprintf("/push/subscriptions?match_id=%d", m), guest, ``); status != 200 {
		t.Errorf("deixar de seguir o jogo: veio %d", status)
	}
	if _, out := call(subs, "GET", "/push/subscriptions", guest, ``); !reflect.DeepEqual(out["matches"], []interface{}{}) {
		t.Errorf("depois de deixar de seguir, matches deveria ser [], veio %v", out["matches"])
	}

	// Teste de push pelo admin, com o Expo falso
	expo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data": [{"status": "ok", "id": "x"}]}`))
	}))
	defer expo.Close()
	t.Setenv("EXPO_PUSH_URL", expo.URL)

	status, out = call(pushTest, "POST", "/admin/push/test", "", `{"device_id": "teste-rota-1"}`, "X-Admin-Key", "chave-de-teste")
	if status != 200 || out["expo_status"] != "ok" {
		t.Errorf("teste de push: esperado 200 com expo_status ok, veio %d %v", status, out)
	}
	if status, _ := call(pushTest, "POST", "/admin/push/test", admin, `{"device_id": "teste-rota-nao-existe"}`); status != 404 {
		t.Errorf("teste de push para aparelho não registrado: esperado 404, veio %d", status)
	}

	// Desligar as notificações apaga tudo o que o aparelho seguia
	if status, _ := call(register, "DELETE", "/push/register", guest, ``); status != 200 {
		t.Errorf("desligar notificações: veio %d", status)
	}
	if _, out := call(subs, "GET", "/push/subscriptions", guest, ``); !reflect.DeepEqual(out["teams"], []interface{}{}) {
		t.Errorf("depois de desligar, teams deveria ser [], veio %v", out["teams"])
	}
}
