package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/models"
	"App-Futebol/testutil"
	"App-Futebol/utils"
	"net/http/httptest"
	"testing"
	"time"
)

// O "atualizar jogos" do admin responde na hora e roda em segundo plano, um por liga de
// cada vez (contra um Postgres de verdade: testutil.OpenDB)
func TestAtualizacaoDoAdminEmSegundoPlano(t *testing.T) {
	database.DB = testutil.OpenDB(t, "matches")
	t.Setenv("JWT_SECRET", "segredo-de-teste")
	admin, _ := utils.GenerateUserToken(1, models.RoleAdmin, "")
	guest, _ := utils.GenerateToken("teste-update")

	iniciou := make(chan string, 4)
	libera := make(chan struct{})
	original := updateLeague
	updateLeague = func(league string) { iniciou <- league; <-libera }
	t.Cleanup(func() { updateLeague = original })

	h := middlewares.JWTAuth(MatchesHandler)
	call := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/matches?league=TSU&update=true", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}

	inicio := time.Now()
	rec := call(admin)
	if rec.Code != 200 || rec.Header().Get("X-Update") != "started" || time.Since(inicio) > 5*time.Second {
		t.Fatalf("esperado 200 na hora com X-Update started, veio %d %q em %s", rec.Code, rec.Header().Get("X-Update"), time.Since(inicio))
	}
	select {
	case l := <-iniciou:
		if l != "TSU" {
			t.Errorf("atualizou a liga errada: %s", l)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a atualização em segundo plano não começou")
	}
	if rec := call(admin); rec.Header().Get("X-Update") != "running" {
		t.Errorf("com uma rodando, a segunda não pode começar outra: %q", rec.Header().Get("X-Update"))
	}
	if rec := call(guest); rec.Header().Get("X-Update") != "" {
		t.Error("convidado não pode disparar atualização")
	}

	close(libera)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, rodando := leagueUpdates.Load("TSU"); !rodando || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rec := call(admin); rec.Header().Get("X-Update") != "started" {
		t.Errorf("depois de terminar, pode atualizar de novo: %q", rec.Header().Get("X-Update"))
	}
	<-iniciou
}
