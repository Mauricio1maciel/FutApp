package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/testutil"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Validação da rota que carrega a temporada de uma liga da ESPN (sem disparar a carga:
// ela buscaria a ESPN de verdade)
func TestAdminSyncESPNSeasonValida(t *testing.T) {
	database.DB = testutil.OpenDB(t, "leagues")
	t.Setenv("JWT_SECRET", "segredo-de-teste")
	t.Setenv("ADMIN_KEY", "chave-de-teste")
	h := middlewares.AdminAuth(AdminSyncESPNSeasonHandler)

	call := func(method, path string) (int, string) {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("X-Admin-Key", "chave-de-teste")
		rec := httptest.NewRecorder()
		h(rec, req)
		var out map[string]string
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out["error"]
	}

	if status, _ := call("GET", "/admin/sync-espn-season?league=UNL"); status != http.StatusMethodNotAllowed {
		t.Errorf("GET: esperado 405, veio %d", status)
	}
	// Liga da football-data: a carga pela ESPN não é para ela
	status, msg := call("POST", "/admin/sync-espn-season?league=BSA")
	if status != http.StatusBadRequest || !strings.Contains(msg, "UNL") {
		t.Errorf("liga que não vem da ESPN: esperado 400 listando as ligas da ESPN, veio %d %q", status, msg)
	}

	req := httptest.NewRequest("POST", "/admin/sync-espn-season?league=UNL", nil)
	rec := httptest.NewRecorder()
	h(rec, req) // sem a chave de admin
	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
		t.Errorf("sem admin: esperado 401/403, veio %d", rec.Code)
	}
}
