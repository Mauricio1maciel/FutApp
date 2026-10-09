// Package testutil abre o banco dos testes de integração com segurança.
// Só é importado por arquivos _test.go.
package testutil

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// OpenDB abre o Postgres de TEST_DATABASE_URL, já com as migrations aplicadas
// (veja migrations/README.md):
//
//	TEST_DATABASE_URL=postgres://postgres:x@localhost:55436/postgres?sslmode=disable go test ./...
//
// Pula o teste se a variável não existir ou se faltar a tabela pedida (migration ainda
// não aplicada). Recusa banco que não seja local: os testes apagam os dados que criam.
func OpenDB(t *testing.T, requiredTable string) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definido: teste de integração pulado")
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1") {
		t.Fatal("TEST_DATABASE_URL precisa ser um banco local: os testes apagam dados")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	var ok bool
	if err := db.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, "public."+requiredTable).Scan(&ok); err != nil || !ok {
		db.Close()
		t.Skipf("banco de teste sem a tabela %s: aplique as migrations", requiredTable)
	}

	// O `go test ./...` roda os pacotes em paralelo e todos dividem este banco: com o
	// lock, os testes de integração rodam um de cada vez (o worker de um teste não
	// envia os avisos criados por outro)
	ctx := context.Background()
	lock, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lock.ExecContext(ctx, `SELECT pg_advisory_lock(hashtext('app-futebol-testes'))`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		lock.ExecContext(ctx, `SELECT pg_advisory_unlock(hashtext('app-futebol-testes'))`)
		lock.Close()
		db.Close()
	})
	return db
}
