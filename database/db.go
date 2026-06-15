package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

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

	// A URL deve ser montada exatamente assim:
	// postgres://usuario:senha@host:port/dbname?params
	// Ajuste a string de conexão para isto:
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&prepareThreshold=0&binary_parameters=yes",
		user, password, host, port, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Erro ao abrir conexão com o banco: %v", err)
	}

	// Testa a conexão
	err = db.Ping()
	if err != nil {
		log.Fatalf("Erro ao pingar o banco: %v", err)
	}

	// Configuração do Pool de Conexões (Crucial para performance no Render/Supabase)
	db.SetMaxOpenConns(25)                 // Define o limite de conexões simultâneas
	db.SetMaxIdleConns(25)                 // Define quantas conexões ficam em espera
	db.SetConnMaxLifetime(5 * time.Minute) // Renova conexões para evitar erros de timeout

	DB = db
	fmt.Println("Banco conectado com sucesso e Pool configurado!")
}
