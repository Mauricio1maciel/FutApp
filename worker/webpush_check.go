package worker

import (
	"App-Futebol/services"
	"App-Futebol/utils"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// checkWebPushKeys confere, ao subir, a chave do push no navegador. O navegador se
// inscreve com a chave pública que a API entrega; se o worker assina com outro par, o
// serviço de push recusa tudo o que ele manda (403) e nenhuma notificação de jogo chega ao
// site, enquanto o teste do admin (que sai da API) funciona e esconde o problema.
func checkWebPushKeys() {
	if !services.WebPushEnabled() {
		return
	}
	problem := ""
	if err := services.CheckVAPIDPair(); err != nil {
		problem = err.Error()
	} else if base := strings.TrimRight(os.Getenv("API_PUBLIC_URL"), "/"); base != "" {
		apiKey, err := fetchWebPushKey(&http.Client{Timeout: 90 * time.Second}, base)
		switch {
		case err != nil:
			utils.CustomLog("JOBS", "Não deu para conferir a chave do push no navegador com a API: %v", err)
			return
		case apiKey != os.Getenv("VAPID_PUBLIC_KEY"):
			problem = "a VAPID_PUBLIC_KEY do worker é diferente da que a API entrega aos navegadores"
		}
	}

	if problem == "" {
		utils.CustomLog("JOBS", "Chave do push no navegador confere com a API ✓")
		return
	}
	msg := "Push no navegador vai falhar: " + problem + ". Use no worker as mesmas três linhas VAPID do Render."
	utils.CustomLog("JOBS", "🚨 %s", msg)
	services.NotifyAdmins(context.Background(), "🚨 Chave do push no navegador", msg)
}

// fetchWebPushKey pega a chave pública que a API entrega aos navegadores
func fetchWebPushKey(client *http.Client, apiBase string) (string, error) {
	resp, err := client.Get(apiBase + "/push/web-key")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusServiceUnavailable {
		return "", fmt.Errorf("a API está sem as chaves VAPID")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("a API respondeu %s", resp.Status)
	}
	var body struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return body.PublicKey, nil
}
