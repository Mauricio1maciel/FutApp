package database

import (
	"App-Futebol/models"
	"context"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"
)

// Tipos de aviso por inscrição: cada um recebe só o que escolheu; quem segue por dois
// caminhos recebe se qualquer um pedir; seguir de novo sem tipos mantém a escolha
func TestPushTiposDeAviso(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	m := newTestMatch(t, 7770501, 0, "TIMED", 0, 0)
	newTestDevice(t, "teste-push-1", "ExponentPushToken[t1]")
	newTestDevice(t, "teste-push-2", "ExponentPushToken[t2]")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	tokens := func(kind string) []string {
		t.Helper()
		tk, err := GetPushTokensForMatch(ctx, m, kind)
		must(err)
		sort.Strings(tk)
		return tk
	}
	both := []string{"ExponentPushToken[t1]", "ExponentPushToken[t2]"}
	only2 := []string{"ExponentPushToken[t2]"}

	must(SubscribeTeam(ctx, "teste-push-1", tstHome, []string{"goal", "end"}, true)) // só gols e fim
	must(SubscribeLeague(ctx, "teste-push-2", tstLeague, nil, false))                // tudo
	if got := tokens("goal"); !reflect.DeepEqual(got, both) {
		t.Errorf("gol: esperado os dois, veio %v", got)
	}
	if got := tokens("start"); !reflect.DeepEqual(got, only2) {
		t.Errorf("início: esperado só quem quer tudo, veio %v", got)
	}

	// O app segue de novo sem mandar tipos (ex: ao abrir): a escolha fica
	must(SubscribeTeam(ctx, "teste-push-1", tstHome, nil, false))
	if got := tokens("start"); !reflect.DeepEqual(got, only2) {
		t.Errorf("seguir de novo sem tipos não pode apagar a escolha, veio %v", got)
	}

	// Seguindo também o jogo (com tudo), passa a receber o início
	must(SubscribeMatch(ctx, "teste-push-1", m, nil, false))
	if got := tokens("start"); !reflect.DeepEqual(got, both) {
		t.Errorf("quem segue por dois caminhos recebe se qualquer um pedir, veio %v", got)
	}

	subs, err := GetPushSubscriptions(ctx, "teste-push-1")
	must(err)
	if got := subs.Kinds.Teams[strconv.FormatInt(tstHome, 10)]; !reflect.DeepEqual(got, []string{"goal", "end"}) {
		t.Errorf("GET: tipos do time, veio %v", got)
	}
	if len(subs.Kinds.Matches) != 0 || !reflect.DeepEqual(subs.AvailableKinds, PushKinds) {
		t.Errorf("GET: jogo com todos não aparece em kinds; available_kinds = todos. Veio %+v", subs)
	}

	// Voltar a receber todos
	must(SubscribeTeam(ctx, "teste-push-1", tstHome, nil, true))
	if subs, _ := GetPushSubscriptions(ctx, "teste-push-1"); len(subs.Kinds.Teams) != 0 {
		t.Errorf("depois de voltar a receber todos, o time não deveria ter tipos, veio %v", subs.Kinds.Teams)
	}

	// O banco recusa tipo desconhecido e lista vazia
	if _, err := DB.Exec(`UPDATE team_subscriptions SET kinds = ARRAY['xyz'] WHERE device_id = 'teste-push-1'`); err == nil {
		t.Error("tipo desconhecido deveria ser recusado pelo banco")
	}
	if _, err := DB.Exec(`UPDATE team_subscriptions SET kinds = ARRAY[]::text[] WHERE device_id = 'teste-push-1'`); err == nil {
		t.Error("lista vazia deveria ser recusada pelo banco")
	}
}

// Cartão vermelho: um aviso por cartão novo, com o lado tirado da escalação quando o
// evento vem sem time; gravar de novo não repete; jogo antigo não vira aviso
func TestPushCartaoVermelho(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	m := newTestMatch(t, 7770601, -30*time.Minute, "IN_PLAY", 1, 0)
	newTestDevice(t, "teste-push-1", "ExponentPushToken[t1]")
	if err := SubscribeMatch(ctx, "teste-push-1", m, nil, false); err != nil {
		t.Fatal(err)
	}

	type aviso struct{ side, player, minute string }
	avisos := func(matchID int64) []aviso {
		t.Helper()
		rows, err := DB.Query(`SELECT goal_side, player, minute FROM push_outbox
                               WHERE match_id = $1 AND kind = 'red_card' ORDER BY id`, matchID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []aviso
		for rows.Next() {
			var a aviso
			rows.Scan(&a.side, &a.player, &a.minute)
			out = append(out, a)
		}
		return out
	}

	jogo := models.ESPNMatchDB{
		MatchID: "9990601", League: tstLeague, Season: "2026", Status: "in",
		MatchDate:      time.Now().UTC().Add(-30 * time.Minute).Format("2006-01-02T15:04Z"),
		ESPNHomeTeamID: tstESPNHome, ESPNAwayTeamID: tstESPNAway, HomeScore: "1", AwayScore: "0",
	}
	lineups := []models.ESPNLineupDB{
		{MatchID: "9990601", ESPNTeamID: tstESPNAway, ESPNPlayerID: 1, PlayerName: "Zagueiro Fora", IsStarter: true},
	}
	events := []models.ESPNEventDB{
		{MatchID: "9990601", Minute: "12'", EventType: "Goal", ESPNTeamID: tstESPNHome, PlayerName: "Atacante Casa"},
		{MatchID: "9990601", Minute: "38'", EventType: "Red Card", PlayerName: "Zagueiro Fora"}, // sem time
	}
	for i := 0; i < 2; i++ { // a segunda gravação (minuto seguinte) não repete o aviso
		if err := SaveFullMatchHistory(ctx, jogo, lineups, events); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := avisos(m), []aviso{{"away", "Zagueiro Fora", "38'"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("primeiro cartão: esperado %v, veio %v", want, got)
	}

	events = append(events, models.ESPNEventDB{MatchID: "9990601", Minute: "70'", EventType: "Red Card",
		ESPNTeamID: tstESPNHome, PlayerName: "Volante Casa"})
	if err := SaveFullMatchHistory(ctx, jogo, lineups, events); err != nil {
		t.Fatal(err)
	}
	if got := avisos(m); len(got) != 2 || got[1] != (aviso{"home", "Volante Casa", "70'"}) {
		t.Errorf("segundo cartão: veio %v", got)
	}

	// Jogo de dois dias atrás (ex: carga de jogos antigos): nada
	old := newTestMatch(t, 7770602, -48*time.Hour, "FINISHED", 0, 0)
	if err := SubscribeMatch(ctx, "teste-push-1", old, nil, false); err != nil {
		t.Fatal(err)
	}
	antigo := jogo
	antigo.MatchID, antigo.Status = "9990602", "post"
	antigo.MatchDate = time.Now().UTC().Add(-48 * time.Hour).Format("2006-01-02T15:04Z")
	if err := SaveFullMatchHistory(ctx, antigo, nil, events); err != nil {
		t.Fatal(err)
	}
	if got := avisos(old); len(got) != 0 {
		t.Errorf("jogo antigo não pode virar aviso, veio %v", got)
	}
}

func TestRecentForPush(t *testing.T) {
	now := time.Date(2026, 10, 18, 22, 0, 0, 0, time.UTC)
	casos := []struct {
		status, date string
		want         bool
	}{
		{"in", "", true},
		{"post", "2026-10-18T19:30Z", true},  // acabou agora há pouco
		{"post", "2026-10-18T17:30Z", false}, // começou há 4h30
		{"post", "2026-10-16T19:30Z", false},
		{"pre", "2026-10-18T21:30Z", false},
	}
	for _, c := range casos {
		if got := recentForPush(models.ESPNMatchDB{Status: c.status, MatchDate: c.date}, now); got != c.want {
			t.Errorf("%s %s: esperado %v, veio %v", c.status, c.date, c.want, got)
		}
	}
}
