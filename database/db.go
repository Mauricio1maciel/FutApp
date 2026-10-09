package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

// Limites de tempo das operações no banco. Sem eles, uma consulta travada
// segurava a requisição até o WriteTimeout do servidor (90s) e mantinha a
// conexão presa no pool.
const (
	queryTimeout = 10 * time.Second // uma consulta avulsa
	txTimeout    = 30 * time.Second // transação, que roda vários comandos
)

// withTimeout limita uma consulta, respeitando o cancelamento de quem chamou:
// se o app desiste da requisição, a consulta é cancelada no banco também.
func withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, queryTimeout)
}

// withTxTimeout é o withTimeout das transações, que precisam de mais folga
func withTxTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, txTimeout)
}

func Connect() {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	// Certifique-se de que o user não está vazio
	if user == "" || host == "" {
		log.Fatal("Variáveis de ambiente do banco não carregadas corretamente!")
	}

	// SSL ligado por padrão: sem ele a senha e os dados trafegam abertos até o Supabase.
	// DB_SSLMODE=disable só para banco local (ex: Postgres no Docker).
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		sslmode = "require"
	}

	// binary_parameters é do lib/pq e evita prepared statements nomeados, necessário
	// atrás do pooler do Supabase. (prepareThreshold é do JDBC: um PostgreSQL comum
	// recusa a conexão.) connect_timeout: sem ele, abrir conexão com o banco fora do
	// ar espera para sempre.
	params := url.Values{}
	params.Set("sslmode", sslmode)
	params.Set("binary_parameters", "yes")
	params.Set("connect_timeout", "5")

	// url.URL escapa usuário e senha: um "@" ou "/" na senha quebrava a string montada à mão
	connURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + dbname,
		RawQuery: params.Encode(),
	}
	connStr := connURL.String()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Erro ao abrir conexão com o banco: %v", err)
	}

	// Testa a conexão
	pingCtx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	err = db.PingContext(pingCtx)
	if err != nil {
		log.Fatalf("Erro ao pingar o banco: %v", err)
	}

	// Configuração do Pool de Conexões (Crucial para performance no Render/Supabase)
	db.SetMaxOpenConns(25)                 // Define o limite de conexões simultâneas
	db.SetMaxIdleConns(25)                 // Define quantas conexões ficam em espera
	db.SetConnMaxLifetime(5 * time.Minute) // Renova conexões para evitar erros de timeout

	DB = db
	fmt.Printf("Banco conectado com sucesso e Pool configurado! (sslmode=%s)\n", sslmode)
}
