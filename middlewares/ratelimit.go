package middlewares

import (
	"App-Futebol/utils"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Limite de pedidos por endereço. Com a versão web, qualquer site pode chamar a API e o
// token de convidado é grátis: sem limite, um script derrubaria o Render (plano grátis).
// Os limites são folgados porque operadoras de celular põem muitos clientes atrás do
// mesmo endereço (CGNAT). RATE_LIMIT=off desliga, sem deploy.

type rateClass struct {
	name   string
	perMin float64 // pedidos por minuto, em média
	burst  float64 // pedidos seguidos antes de começar a esperar
}

var (
	classAuth    = rateClass{"auth", 20, 10}    // criar token e login
	classHeavy   = rateClass{"pesadas", 60, 30} // consultas que dão mais trabalho ao banco
	classDefault = rateClass{"geral", 240, 120} // o resto
)

var heavyRoutes = map[string]bool{
	"/search": true, "/player/stats": true, "/team/stats": true, "/h2h": true, "/standings/chances": true,
}

// routeClass escolhe o limite pela rota. Sem limite: /health (o Orange Pi chama para a
// API não dormir) e o preflight do navegador, que o CORS já respondeu antes daqui.
func routeClass(r *http.Request) (rateClass, bool) {
	switch {
	case r.Method == http.MethodOptions, r.URL.Path == "/health":
		return rateClass{}, false
	case strings.HasPrefix(r.URL.Path, "/auth/"):
		return classAuth, true
	case heavyRoutes[r.URL.Path], r.URL.Path == "/standings" && r.URL.Query().Get("live") == "true":
		return classHeavy, true
	}
	return classDefault, true
}

type bucket struct {
	tokens float64
	last   time.Time
	denied bool // o último pedido foi recusado (para logar só o primeiro de uma sequência)
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{buckets: map[string]*bucket{}, now: time.Now}
}

// allow gasta um pedido do balde (endereço + classe). Sem saldo, diz quanto esperar.
func (l *rateLimiter) allow(key string, c rateClass) (ok bool, remaining int, retryAfter time.Duration, firstDenied bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	perSec := c.perMin / 60
	b, found := l.buckets[key]
	if !found {
		b = &bucket{tokens: c.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = math.Min(c.burst, b.tokens+now.Sub(b.last).Seconds()*perSec)
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		b.denied = false
		return true, int(b.tokens), 0, false
	}
	firstDenied = !b.denied
	b.denied = true
	wait := time.Duration(math.Ceil((1-b.tokens)/perSec)) * time.Second
	return false, 0, wait, firstDenied
}

// cleanup apaga os baldes parados há mais de 10 minutos (já estariam cheios de novo)
func (l *rateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.buckets {
		if l.now().Sub(b.last) > 10*time.Minute {
			delete(l.buckets, k)
		}
	}
}

var limiter = newRateLimiter()

func init() {
	go func() {
		for range time.Tick(time.Minute) {
			limiter.cleanup()
		}
	}()
}

var warnedNoClientIP sync.Once

// RateLimit aplica o limite de pedidos por endereço. Fica dentro do CORS, para a resposta
// 429 também sair com os headers do CORS (o site precisa conseguir lê-la).
func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		class, limited := routeClass(r)
		if !limited || strings.EqualFold(os.Getenv("RATE_LIMIT"), "off") {
			next.ServeHTTP(w, r)
			return
		}
		ip := ClientIP(r)
		if !isPublicIP(ip) {
			// Endereço interno: não dá para saber quem é o cliente (ex: a configuração do
			// proxy mudou). Limitar juntaria todo mundo num balde só
			warnedNoClientIP.Do(func() {
				utils.CustomLog("RATE_LIMIT", "Endereço do cliente não identificado (%s): limite desligado para esses pedidos", ip)
			})
			next.ServeHTTP(w, r)
			return
		}

		ok, remaining, wait, first := limiter.allow(class.name+"|"+ip, class)
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(int(class.perMin)))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !ok {
			secs := int(wait.Seconds())
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			if first {
				utils.CustomLog("RATE_LIMIT", "Limite %s atingido: %s %s", class.name, ip, r.URL.Path)
			}
			utils.WriteError(w, http.StatusTooManyRequests,
				fmt.Sprintf("Muitos pedidos em pouco tempo. Tente de novo em %d s.", secs))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP é o endereço de quem fez o pedido. Na frente do Render há a Cloudflare, que
// põe o endereço real em CF-Connecting-IP (o cliente não consegue falsificar: a Cloudflare
// sobrescreve). O X-Forwarded-For fica de reserva, porque o próprio cliente pode mandar
// um falso; sem proxy nenhum (rodando local), vale o endereço da conexão.
func ClientIP(r *http.Request) string {
	for _, h := range []string{"CF-Connecting-IP", "True-Client-IP"} {
		if v := strings.TrimSpace(r.Header.Get(h)); net.ParseIP(v) != nil {
			return v
		}
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if v := strings.TrimSpace(strings.Split(fwd, ",")[0]); net.ParseIP(v) != nil {
			return v
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isPublicIP(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsUnspecified() &&
		!ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()
}
