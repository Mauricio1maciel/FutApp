package handlers

import (
	"App-Futebol/database"
	"App-Futebol/middlewares"
	"App-Futebol/services"
	"App-Futebol/testutil"
	"App-Futebol/utils"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Registro do navegador e chave pública do push no navegador (contra um Postgres de
// verdade: testutil.OpenDB)
func TestRegistroDoNavegador(t *testing.T) {
	database.DB = testutil.OpenDB(t, "push_devices")
	t.Setenv("JWT_SECRET", "segredo-de-teste")
	limpa := func() { database.DB.Exec(`DELETE FROM push_devices WHERE device_id LIKE 'teste-nav-%'`) }
	limpa()
	t.Cleanup(limpa)
	guest, _ := utils.GenerateToken("teste-nav-1")

	call := func(h http.HandlerFunc, method, path, token, body string) (int, map[string]interface{}) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		var out map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	register := middlewares.JWTAuth(PushRegisterHandler)
	key := func(n int) string {
		b := make([]byte, n)
		rand.Read(b)
		if n == 65 {
			b[0] = 4 // ponto não comprimido
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	sub := func(endpoint, p256dh, auth string) string {
		return fmt.Sprintf(`{"platform": "web", "subscription": {"endpoint": %q, "keys": {"p256dh": %q, "auth": %q}}}`,
			endpoint, p256dh, auth)
	}
	chrome := "https://fcm.googleapis.com/fcm/send/abc:xyz"

	// Sem as chaves VAPID no servidor: 503 nas duas rotas
	t.Setenv("VAPID_PUBLIC_KEY", "")
	if status, _ := call(PushWebKeyHandler, "GET", "/push/web-key", "", ``); status != 503 {
		t.Errorf("chave pública sem VAPID: esperado 503, veio %d", status)
	}
	if status, _ := call(register, "POST", "/push/register", guest, sub(chrome, key(65), key(16))); status != 503 {
		t.Errorf("registro sem VAPID: esperado 503, veio %d", status)
	}

	priv, pub, _ := services.GenerateVAPIDKeys()
	t.Setenv("VAPID_PUBLIC_KEY", pub)
	t.Setenv("VAPID_PRIVATE_KEY", priv)
	t.Setenv("VAPID_SUBJECT", "mailto:teste@example.com")
	if status, out := call(PushWebKeyHandler, "GET", "/push/web-key", "", ``); status != 200 || out["public_key"] != pub {
		t.Errorf("chave pública: veio %d %v", status, out)
	}

	casos := []struct {
		nome   string
		body   string
		status int
	}{
		{"Chrome", sub(chrome, key(65), key(16)), 200},
		{"Firefox", sub("https://updates.push.services.mozilla.com/wpush/v2/abc", key(65), key(16)), 200},
		{"Safari", sub("https://web.push.apple.com/QAbc", key(65), key(16)), 200},
		{"Edge", sub("https://wns2-par02p.notify.windows.com/w/?token=abc", key(65), key(16)), 200},
		{"endereço qualquer", sub("https://servidor-qualquer.com/push", key(65), key(16)), 400},
		{"endereço interno", sub("https://localhost/push", key(65), key(16)), 400},
		{"sem https", sub("http://fcm.googleapis.com/fcm/send/abc", key(65), key(16)), 400},
		{"domínio parecido", sub("https://fcm.googleapis.com.evil.com/x", key(65), key(16)), 400},
		{"chave p256dh curta", sub(chrome, key(16), key(16)), 400},
		{"chave auth errada", sub(chrome, key(65), "não-é-base64!"), 400},
		{"sem subscription", `{"platform": "web"}`, 400},
	}
	for _, c := range casos {
		if status, out := call(register, "POST", "/push/register", guest, c.body); status != c.status {
			t.Errorf("%s: esperado %d, veio %d %v", c.nome, c.status, status, out)
		}
	}

	// Gravado como navegador, com as chaves
	target, err := database.GetPushTarget(t.Context(), "teste-nav-1")
	if err != nil || !target.IsWeb() || target.P256dh == "" || target.Auth == "" ||
		!strings.HasPrefix(target.Token, "https://wns2-par02p.notify.windows.com/") {
		t.Errorf("navegador gravado: %+v (%v)", target, err)
	}

	// O app no mesmo aparelho volta a ser app (as chaves do navegador saem)
	if status, _ := call(register, "POST", "/push/register", guest, `{"token": "ExponentPushToken[nav]", "platform": "android"}`); status != 200 {
		t.Fatalf("registro do app: veio %d", status)
	}
	if target, _ := database.GetPushTarget(t.Context(), "teste-nav-1"); target.IsWeb() || target.P256dh != "" {
		t.Errorf("depois do registro do app, o aparelho deveria ser app: %+v", target)
	}
}
