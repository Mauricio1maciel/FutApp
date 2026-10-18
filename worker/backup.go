package worker

import (
	"App-Futebol/utils"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// loopBackup copia o banco toda madrugada às 04:00, depois da sincronização das 03:00.
// Usuários, aparelhos e o que cada um segue não vêm de API nenhuma: sem cópia, se
// perderiam junto com o banco.
func loopBackup() {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		time.Sleep(time.Until(next))

		file, err := RunBackup(context.Background())
		if err != nil {
			utils.CustomLog("JOBS", "Erro no backup do banco: %v", err)
			continue
		}
		utils.CustomLog("JOBS", "Backup do banco gravado: %s", file)
	}
}

// RunBackup grava uma cópia do banco com o pg_dump (formato custom, compactado; só o
// schema public, que são as nossas tabelas) e apaga as mais antigas, mantendo
// BACKUP_KEEP (padrão 7). Pasta: BACKUP_DIR (padrão ~/backups/futapp).
func RunBackup(ctx context.Context) (string, error) {
	pgDump, err := exec.LookPath(envOr("PG_DUMP_PATH", "pg_dump"))
	if err != nil {
		return "", fmt.Errorf("pg_dump não encontrado: instale o postgresql-client-17 (docs/OPERACAO.md)")
	}
	dir := envOr("BACKUP_DIR", "")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, "backups", "futapp")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	file := filepath.Join(dir, "futapp-"+time.Now().Format("2006-01-02")+".dump")
	partial := file + ".parcial" // só vira .dump se o pg_dump terminar bem

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, pgDump,
		"--format=custom", "--schema=public", "--no-owner", "--no-privileges", "--file="+partial)
	cmd.Env = append(os.Environ(),
		"PGHOST="+os.Getenv("DB_HOST"),
		"PGPORT="+os.Getenv("DB_PORT"),
		"PGUSER="+os.Getenv("DB_USER"),
		"PGPASSWORD="+os.Getenv("DB_PASSWORD"),
		"PGDATABASE="+os.Getenv("DB_NAME"),
		"PGSSLMODE="+envOr("DB_SSLMODE", "require"),
		"PGCONNECT_TIMEOUT=15",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(partial)
		return "", fmt.Errorf("pg_dump: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if err := os.Rename(partial, file); err != nil {
		return "", err
	}

	keep := 7
	if n, err := strconv.Atoi(os.Getenv("BACKUP_KEEP")); err == nil && n > 0 {
		keep = n
	}
	pruneBackups(dir, keep)
	return file, nil
}

// pruneBackups apaga as cópias mais antigas (o nome tem a data, então a ordem do nome é a
// da data) e as que ficaram pela metade
func pruneBackups(dir string, keep int) {
	partials, _ := filepath.Glob(filepath.Join(dir, "futapp-*.dump.parcial"))
	for _, p := range partials {
		os.Remove(p)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "futapp-*.dump"))
	sort.Strings(files)
	for len(files) > keep {
		os.Remove(files[0])
		files = files[1:]
	}
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
