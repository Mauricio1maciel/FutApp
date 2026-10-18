package worker

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPruneBackups(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{
		"futapp-2026-10-01.dump", "futapp-2026-10-03.dump", "futapp-2026-10-02.dump",
		"futapp-2026-10-04.dump", "futapp-2026-10-05.dump.parcial", "outro-arquivo.txt",
	} {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600)
	}
	pruneBackups(dir, 2)

	entries, _ := os.ReadDir(dir)
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	want := []string{"futapp-2026-10-03.dump", "futapp-2026-10-04.dump", "outro-arquivo.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("esperado %v, veio %v", want, got)
	}
}

// Backup de verdade contra o Postgres de teste (TEST_DATABASE_URL) com o pg_dump instalado
func TestRunBackup(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL não definido")
	}
	pgDump := "/usr/lib/postgresql/17/bin/pg_dump"
	if _, err := os.Stat(pgDump); err != nil {
		if pgDump, err = exec.LookPath("pg_dump"); err != nil {
			t.Skip("pg_dump não instalado")
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	pass, _ := u.User.Password()
	dir := t.TempDir()
	for k, v := range map[string]string{
		"PG_DUMP_PATH": pgDump, "BACKUP_DIR": dir, "BACKUP_KEEP": "7",
		"DB_HOST": u.Hostname(), "DB_PORT": u.Port(), "DB_USER": u.User.Username(),
		"DB_PASSWORD": pass, "DB_NAME": u.Path[1:], "DB_SSLMODE": "disable",
	} {
		t.Setenv(k, v)
	}

	file, err := RunBackup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil || info.Size() == 0 || filepath.Dir(file) != dir {
		t.Fatalf("backup não gravado direito: %s (%v)", file, err)
	}
	if partials, _ := filepath.Glob(filepath.Join(dir, "*.parcial")); len(partials) > 0 {
		t.Errorf("sobrou arquivo pela metade: %v", partials)
	}
}
