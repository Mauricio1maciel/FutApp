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

// open monta a conexão a partir do .env. O sql.Open ainda não conecta: quem testa é o ping
func open() (*sql.DB, string) {
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

	return db, sslmode
}

func ping(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	return db.PingContext(ctx)
}

// Connect abre o banco e encerra o programa se ele não responder. É o dos comandos que
// rodam e saem (ex: -backup, -criar-admin), em que esperar não ajuda ninguém
func Connect() {
	db, sslmode := open()
	if err := ping(db); err != nil {
		log.Fatalf("Erro ao pingar o banco: %v", err)
	}
	finishConnect(db, sslmode)
}

// ConnectWaiting abre o banco e, se ele não responder, tenta de novo em vez de encerrar.
// Depois de uma queda de energia, o Orange Pi pode subir antes da internet: com o Connect,
// o worker morria na largada e só voltava quando alguém o reiniciasse.
func ConnectWaiting() {
	db, sslmode := open()
	waitForPing(func() error { return ping(db) }, time.Sleep)
	finishConnect(db, sslmode)
}

// waitForPing tenta até o banco responder, esperando 5 s, 10 s, 20 s... até 1 minuto
// entre as tentativas
func waitForPing(ping func() error, sleep func(time.Duration)) {
	wait := 5 * time.Second
	for {
		err := ping()
		if err == nil {
			return
		}
		log.Printf("Banco fora do ar (%v): nova tentativa em %s", err, wait)
		sleep(wait)
		wait = min(2*wait, time.Minute)
	}
}

func finishConnect(db *sql.DB, sslmode string) {
	// Pool de conexões. Abrir uma conexão do Render (EUA) até o pooler do Supabase (São
	// Paulo) custa ~4 idas e voltas (TCP, TLS e autenticação), uns 0,8 s. Com 5 minutos de
	// vida, cada conexão era refeita 12 vezes por hora, e o pedido que pegava a conexão
	// nova pagava a espera. Conexão parada há 15 minutos é fechada; a que o pooler derrubar
	// é refeita sozinha pelo database/sql (o lib/pq a marca como ruim e o pedido tenta de novo).
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(15 * time.Minute)

	DB = db
	fmt.Printf("Banco conectado com sucesso e Pool configurado! (sslmode=%s)\n", sslmode)
}
