package main

import (
	"App-Futebol/database"
	"App-Futebol/database/servico"
	"App-Futebol/handlers"
	"App-Futebol/middlewares"
	"App-Futebol/services"

	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Println("Aviso: Arquivo .env não encontrado. Usando variáveis do sistema.")
	}

	database.Connect()

	services.StartBackgroundUpdater()
	servico.StartBackgroundScheduler()

	http.HandleFunc("/auth/guest", handlers.GuestAuthHandler)

	http.HandleFunc("/search", middlewares.JWTAuth(handlers.GlobalSearchHandler))
	http.HandleFunc("/details", middlewares.JWTAuth(handlers.DetailsHandler))

	http.HandleFunc("/teams", middlewares.JWTAuth(handlers.TeamsHandler))

	http.HandleFunc("/seasons", middlewares.JWTAuth(handlers.SeasonsHandler))

	http.HandleFunc("/matches", middlewares.JWTAuth(handlers.MatchesHandler))
	http.HandleFunc("/team/matches", middlewares.JWTAuth(handlers.TeamMatchesHandler))

	http.HandleFunc("/matches/calendar", middlewares.JWTAuth(handlers.CalendarHandler))

	http.HandleFunc("/standings", middlewares.JWTAuth(handlers.StandingsHandler))

	http.HandleFunc("/players", middlewares.JWTAuth(handlers.PlayersHandler))

	http.HandleFunc("/league/stats", middlewares.JWTAuth(handlers.LeagueStatsHandler))

	http.HandleFunc("/team/players", middlewares.JWTAuth(handlers.TeamPlayersHandler))

	http.HandleFunc("/matches/live", middlewares.JWTAuth(handlers.LiveMatchesHandler))
	http.HandleFunc("/match/history", middlewares.JWTAuth(handlers.MatchHistoryHandler))
	http.HandleFunc("/match_history_old", middlewares.JWTAuth(handlers.SyncPastMatchHandler))
	http.HandleFunc("/team/players_espn", middlewares.JWTAuth(handlers.SyncESPNTeamHandler))

	// 🛡️ ROTAS ADMIN (Também protegidas para ninguém acionar as rotinas indevidamente)
	http.HandleFunc("/admin/sync-teams", middlewares.JWTAuth(handlers.SyncTeamsHandler))
	http.HandleFunc("/admin/force-sync", middlewares.JWTAuth(handlers.ForceSyncHistoryHandler))

	http.HandleFunc("/admin/run-image-bot", middlewares.JWTAuth(func(w http.ResponseWriter, r *http.Request) {
		go services.RunImageBot()
		w.Write([]byte("Robô de imagens iniciado em background!"))
	}))

	porta := os.Getenv("PORT")
	if porta == "" {
		porta = "5000"
	}

	fmt.Printf("API Futebol rodando na porta %s...\n", porta)
	log.Fatal(http.ListenAndServe(":"+porta, nil))
}
