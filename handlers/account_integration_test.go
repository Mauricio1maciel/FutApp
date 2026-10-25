package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/testutil"
	"App-Futebol/utils"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// Contas: cadastro levando o que o convidado seguia, login em outro aparelho, avisos nos
// dois, sair e apagar a conta
func TestContas(t *testing.T) {
	database.DB = testutil.OpenDB(t, "push_outbox")
	t.Setenv("JWT_SECRET", "segredo-de-teste")
	ctx := context.Background()

	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	limpa := func() {
		exec(`DELETE FROM users WHERE email LIKE '%@conta-teste.dev'`)
		exec(`DELETE FROM matches WHERE league = 'TSC'`)
		exec(`DELETE FROM push_devices WHERE device_id LIKE 'teste-conta-%'`)
		exec(`DELETE FROM teams WHERE api_id IN (994001, 994002)`)
		exec(`DELETE FROM leagues WHERE code_api = 'TSC'`)
	}
	limpa()
	t.Cleanup(limpa)

	exec(`INSERT INTO teams (api_id, espn_team_id, name) VALUES (994001, 884001, 'Casa FC'), (994002, 884002, 'Fora FC')`)
	exec(`INSERT INTO leagues (code_api, name) VALUES ('TSC', 'Liga de Teste')`)
	var m int64
	if err := database.DB.QueryRow(`
        INSERT INTO matches (id_event, league, season, round, api_home_team_id, api_away_team_id,
                             home_score, away_score, match_date, status)
        VALUES (7790101, 'TSC', '2026', 1, 994001, 994002, 0, 0, now(), 'TIMED') RETURNING id`).Scan(&m); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"teste-conta-1", "teste-conta-2"} {
		if err := database.RegisterPushDevice(ctx, d, "ExponentPushToken["+d+"]", "android"); err != nil {
			t.Fatal(err)
		}
	}
	// Como convidado, o aparelho 1 segue o time da casa e a liga
	if err := database.SubscribeTeam(ctx, database.Device("teste-conta-1"), 994001, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := database.SubscribeLeague(ctx, database.Device("teste-conta-1"), "TSC", []string{"goal"}, true); err != nil {
		t.Fatal(err)
	}

	call := func(h http.HandlerFunc, method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}
	var conta struct {
		Token string `json:"token"`
		User  struct {
			ID          int64  `json:"id"`
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
		} `json:"user"`
	}

	// Validações do cadastro
	for _, body := range []string{
		`{"email":"sem-arroba","password":"12345678"}`,
		`{"email":"a@conta-teste.dev","password":"curta"}`,
	} {
		if rec := call(RegisterHandler, http.MethodPost, "/auth/register", "", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: esperava 400, veio %d", body, rec.Code)
		}
	}

	guest1, _ := utils.GenerateToken("teste-conta-1")
	guest2, _ := utils.GenerateToken("teste-conta-2")
	// Ligar um aparelho sem o token de convidado dele é recusado (e a conta não é criada)
	if rec := call(RegisterHandler, http.MethodPost, "/auth/register", guest2,
		`{"email":"ana@conta-teste.dev","password":"senha-forte","device_id":"teste-conta-1"}`); rec.Code != http.StatusForbidden {
		t.Errorf("aparelho de outro: esperava 403, veio %d", rec.Code)
	}

	rec := call(RegisterHandler, http.MethodPost, "/auth/register", guest1,
		`{"email":" Ana@Conta-Teste.dev ","password":"senha-forte","display_name":"Ana","device_id":"teste-conta-1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("cadastro: %d %s", rec.Code, rec.Body)
	}
	json.Unmarshal(rec.Body.Bytes(), &conta)
	if conta.Token == "" || conta.User.Email != "ana@conta-teste.dev" || conta.User.DisplayName != "Ana" {
		t.Fatalf("resposta do cadastro: %s", rec.Body)
	}
	if rec := call(RegisterHandler, http.MethodPost, "/auth/register", "",
		`{"email":"ana@conta-teste.dev","password":"outra-senha"}`); rec.Code != http.StatusConflict {
		t.Errorf("e-mail repetido: esperava 409, veio %d", rec.Code)
	}

	// O que o convidado seguia agora é da conta, com os tipos de aviso
	subs, err := database.GetPushSubscriptions(ctx, database.User(conta.User.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(subs.Teams) != 1 || subs.Teams[0] != 994001 || len(subs.Leagues) != 1 || subs.Leagues[0] != "TSC" {
		t.Fatalf("a conta não herdou as inscrições: %+v", subs)
	}
	if k := subs.Kinds.Leagues["TSC"]; len(k) != 1 || k[0] != "goal" {
		t.Errorf("tipos da liga perdidos: %v", subs.Kinds.Leagues)
	}
	if left, _ := database.GetPushSubscriptions(ctx, database.Device("teste-conta-1")); len(left.Teams)+len(left.Leagues) != 0 {
		t.Errorf("o aparelho ficou com inscrições de convidado: %+v", left)
	}

	// Pelo token da conta, /push/subscriptions mostra o da conta
	subsHandler := middlewares.JWTAuth(PushSubscriptionsHandler)
	if rec := call(subsHandler, http.MethodGet, "/push/subscriptions", conta.Token, ""); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "994001") {
		t.Errorf("/push/subscriptions da conta: %d %s", rec.Code, rec.Body)
	}

	// Entra no aparelho 2: os avisos do time vão para os dois
	login := call(LoginHandler, http.MethodPost, "/auth/login", guest2,
		`{"email":"ana@conta-teste.dev","password":"senha-forte","device_id":"teste-conta-2"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login: %d %s", login.Code, login.Body)
	}
	var segunda struct {
		Token string `json:"token"`
	}
	json.Unmarshal(login.Body.Bytes(), &segunda)
	targets := func(kind string) []string {
		t.Helper()
		ts, err := database.GetPushTargetsForMatch(ctx, m, kind)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, x := range ts {
			out = append(out, x.Token)
		}
		sort.Strings(out)
		return out
	}
	if got := targets("start"); len(got) != 2 {
		t.Fatalf("avisos da conta deviam ir para os 2 aparelhos: %v", got)
	}

	// /me e /auth/refresh
	me := middlewares.JWTAuth(MeHandler)
	if rec := call(me, http.MethodGet, "/me", segunda.Token, ""); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "ana@conta-teste.dev") {
		t.Errorf("/me: %d %s", rec.Code, rec.Body)
	}
	guest, _ := utils.GenerateToken("teste-conta-9")
	if rec := call(me, http.MethodGet, "/me", guest, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("/me de convidado: esperava 401, veio %d", rec.Code)
	}
	if rec := call(middlewares.JWTAuth(RefreshHandler), http.MethodPost, "/auth/refresh", segunda.Token, ""); rec.Code != http.StatusOK {
		t.Errorf("refresh: %d %s", rec.Code, rec.Body)
	}

	// Sai do aparelho 2: ele para de receber
	if rec := call(middlewares.JWTAuth(LogoutHandler), http.MethodPost, "/auth/logout", segunda.Token, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", rec.Code)
	}
	if got := targets("start"); len(got) != 1 || got[0] != "ExponentPushToken[teste-conta-1]" {
		t.Errorf("depois de sair, só o aparelho 1 recebe: %v", got)
	}

	// Apagar a conta: senha errada não apaga; com a certa, some com as inscrições
	if rec := call(me, http.MethodDelete, "/me", conta.Token, `{"password":"errada"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("apagar com senha errada: esperava 401, veio %d", rec.Code)
	}
	if rec := call(me, http.MethodDelete, "/me", conta.Token, `{"password":"senha-forte"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("apagar: %d %s", rec.Code, rec.Body)
	}
	if got := targets("start"); len(got) != 0 {
		t.Errorf("conta apagada, ninguém devia receber: %v", got)
	}
	if rec := call(me, http.MethodGet, "/me", conta.Token, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("token de conta apagada: esperava 401, veio %d", rec.Code)
	}
}
