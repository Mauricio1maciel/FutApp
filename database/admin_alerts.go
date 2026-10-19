package database

import (
	"context"
	"database/sql"
	"errors"
)

// GetDatabaseSize devolve o tamanho do banco em bytes: o mesmo número que o painel do
// Supabase compara com o limite do plano
func GetDatabaseSize(ctx context.Context) (int64, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var size int64
	err := DB.QueryRowContext(ctx, `SELECT pg_database_size(current_database())`).Scan(&size)
	return size, err
}

type TableSize struct {
	Name  string
	Bytes int64
}

// GetLargestTables devolve as maiores tabelas (dados + índices), da maior para a menor
func GetLargestTables(ctx context.Context, limit int) ([]TableSize, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        SELECT c.relname, pg_total_relation_size(c.oid)
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public' AND c.relkind = 'r'
        ORDER BY 2 DESC
        LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := []TableSize{}
	for rows.Next() {
		var t TableSize
		if err := rows.Scan(&t.Name, &t.Bytes); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

// MarkAdminAlertDevice faz o aparelho receber os alertas do servidor (ex: banco enchendo)
func MarkAdminAlertDevice(ctx context.Context, deviceID string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `UPDATE push_devices SET admin_alerts = true WHERE device_id = $1`, deviceID)
	return err
}

// GetAdminPushTargets devolve os aparelhos (app ou navegador) que recebem os alertas do
// servidor
func GetAdminPushTargets(ctx context.Context) ([]PushTarget, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx,
		`SELECT `+pushTargetColumns+` FROM push_devices d WHERE d.admin_alerts ORDER BY d.device_id`)
	if err != nil {
		return nil, err
	}
	return scanPushTargets(rows)
}

// GetAlertState devolve o último valor guardado de um alerta (found = false se nunca houve)
func GetAlertState(ctx context.Context, key string) (value int64, found bool, err error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	err = DB.QueryRowContext(ctx, `SELECT value FROM alert_state WHERE key = $1`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return value, err == nil, err
}

func SetAlertState(ctx context.Context, key string, value int64) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `
        INSERT INTO alert_state (key, value) VALUES ($1, $2)
        ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, key, value)
	return err
}
