package main

import (
	"App-Futebol/database"
	"App-Futebol/handlers"
	"App-Futebol/middlewares"
	"App-Futebol/models"
	"App-Futebol/services"
	"App-Futebol/utils"
	"App-Futebol/worker"
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Println("Aviso: Arquivo .env não encontrado. Usando variáveis do sistema.")
	}

	database.Connect()
	// 2. Cria a "Chave" de inicialização
	isWorker := flag.Bool("worker", false, "Rodar em modo Worker (Orange Pi)")
	criarAdmin := flag.String("criar-admin", "", "Cria (ou troca a senha de) um usuário admin: -criar-admin seu@email.com")
	flag.Parse()

	if *criarAdmin != "" {
		if err := createAdminUser(*criarAdmin); err != nil {
			log.Fatalf("Erro ao criar admin: %v", err)
		}
		return
	}

	// 3. Se for o Orange Pi, roda o Worker e ignora a API!
	if *isWorker {
		utils.CustomLog("SISTEMA", "Iniciando em MODO WORKER no Orange Pi...")
		worker.StartEngine()
		return // Segura o processo aqui para sempre
	}

	// 4. Se não tiver a flag (Render), sobe a API Web normalmente
	utils.CustomLog("SISTEMA", "Iniciando em MODO API WEB...")

	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("JWT_SECRET não configurado! Defina a variável de ambiente antes de subir a API.")
	}

	// A API web só lê do banco: as sincronizações com ESPN e Football-Data rodam no worker.
	// Exceções: /matches/live (ESPN com cache de 30s) e ações de admin.

	mux := http.NewServeMux()

	mux.HandleFunc("/health", handlers.HealthHandler)

	mux.HandleFunc("/auth/guest", handlers.GuestAuthHandler)
	mux.HandleFunc("/auth/login", handlers.LoginHandler)

	mux.HandleFunc("/search", middlewares.JWTAuth(handlers.GlobalSearchHandler))
	mux.HandleFunc("/details", middlewares.JWTAuth(handlers.DetailsHandler))

	mux.HandleFunc("/teams", middlewares.JWTAuth(handlers.TeamsHandler))

	mux.HandleFunc("/seasons", middlewares.JWTAuth(handlers.SeasonsHandler))

	mux.HandleFunc("/matches", middlewares.JWTAuth(handlers.MatchesHandler))
	mux.HandleFunc("/team/matches", middlewares.JWTAuth(handlers.TeamMatchesHandler))

	mux.HandleFunc("/matches/calendar", middlewares.JWTAuth(handlers.CalendarHandler))

	mux.HandleFunc("/standings", middlewares.JWTAuth(handlers.StandingsHandler))

	mux.HandleFunc("/players", middlewares.JWTAuth(handlers.PlayersHandler))

	mux.HandleFunc("/league/stats", middlewares.JWTAuth(handlers.LeagueStatsHandler))

	mux.HandleFunc("/team/players", middlewares.JWTAuth(handlers.TeamPlayersHandler))

	mux.HandleFunc("/matches/live", middlewares.JWTAuth(handlers.LiveMatchesHandler))
	mux.HandleFunc("/match/history", middlewares.JWTAuth(handlers.MatchHistoryHandler))

	// 🛡️ ROTAS ADMIN: usuário admin logado (/auth/login) ou header X-Admin-Key
	mux.HandleFunc("/match_history_old", middlewares.AdminAuth(handlers.SyncPastMatchHandler))
	mux.HandleFunc("/team/players_espn", middlewares.AdminAuth(handlers.SyncESPNTeamHandler))
	mux.HandleFunc("/admin/sync-teams", middlewares.AdminAuth(handlers.SyncTeamsHandler))
	mux.HandleFunc("/admin/force-sync", middlewares.AdminAuth(handlers.ForceSyncHistoryHandler))

	mux.HandleFunc("/admin/sync-daily", middlewares.AdminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !worker.StartDailySync() {
			utils.WriteError(w, http.StatusConflict, "Sincronização diária já está rodando.")
			return
		}
		utils.WriteJSON(w, http.StatusAccepted, map[string]string{"message": "Sincronização diária iniciada em background (times, elencos e vínculos ESPN)."})
	}))

	mux.HandleFunc("/admin/run-image-bot", middlewares.AdminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !services.StartImageBot() {
			utils.WriteError(w, http.StatusConflict, "Robô de imagens já está rodando.")
			return
		}
		utils.WriteJSON(w, http.StatusAccepted, map[string]string{"message": "Robô de imagens iniciado em background!"})
	}))

	// Qualquer rota não cadastrada: 404 no mesmo formato JSON do resto da API
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		utils.WriteError(w, http.StatusNotFound, "Rota não encontrada")
	})

	porta := os.Getenv("PORT")
	if porta == "" {
		porta = "5000"
	}

	server := &http.Server{
		Addr:    ":" + porta,
		Handler: middlewares.RequestLogger(middlewares.Recover(mux)),

		// Sem timeouts, um cliente lento segura a conexão para sempre
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		// Folga para ações de admin que chamam a ESPN/football-data na hora
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// O Render manda SIGTERM no deploy/restart; Ctrl+C manda SIGINT
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		fmt.Printf("API Futebol rodando na porta %s...\n", porta)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro no servidor HTTP: %v", err)
		}
	}()

	<-ctx.Done()
	utils.CustomLog("SISTEMA", "Sinal de parada recebido. Terminando requisições em andamento...")

	// Para de aceitar conexões novas e espera as atuais terminarem (até 20s)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		utils.CustomLog("SISTEMA", "Desligamento forçado: %v", err)
	}
	database.DB.Close()
	utils.CustomLog("SISTEMA", "API encerrada.")
}

// createAdminUser cria um usuário admin pelo terminal (ou troca a senha, se o e-mail já existir).
// A senha é digitada sem aparecer na tela e só o hash bcrypt vai para o banco.
func createAdminUser(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return fmt.Errorf("e-mail inválido: %s", email)
	}

	password, err := readPassword("Senha (mínimo 10 caracteres): ")
	if err != nil {
		return err
	}
	if len(password) < 10 {
		return fmt.Errorf("a senha precisa ter pelo menos 10 caracteres")
	}

	confirm, err := readPassword("Confirme a senha: ")
	if err != nil {
		return err
	}
	if password != confirm {
		return fmt.Errorf("as senhas não conferem")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	id, err := database.UpsertUser(context.Background(), email, string(hash), models.RoleAdmin)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Admin %s pronto (id %d). Faça login em POST /auth/login.\n", email, id)
	return nil
}

// Um leitor só para toda a execução: o bufio lê adiantado, e um leitor novo
// por pergunta perderia a segunda linha quando a senha vem por pipe
var stdinReader = bufio.NewReader(os.Stdin)

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	defer fmt.Println()

	// Terminal de verdade: não mostra a senha. Entrada redirecionada (pipe): lê a linha.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		return string(b), err
	}
	line, err := stdinReader.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}
