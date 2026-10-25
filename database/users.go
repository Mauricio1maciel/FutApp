package database

import (
	"App-Futebol/models"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

func GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var u models.User
	err := DB.QueryRowContext(ctx,
		`SELECT id, email, password_hash, role FROM users WHERE email = $1`,
		strings.ToLower(strings.TrimSpace(email)),
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role)
	return u, err
}

// UpsertUser cria o usuário ou, se o e-mail já existir, troca a senha e o papel
func UpsertUser(ctx context.Context, email string, passwordHash string, role string) (int64, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var id int64
	err := DB.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO UPDATE
		SET password_hash = EXCLUDED.password_hash,
		    role = EXCLUDED.role
		RETURNING id`,
		strings.ToLower(strings.TrimSpace(email)), passwordHash, role,
	).Scan(&id)
	return id, err
}

func TouchUserLogin(ctx context.Context, userID int64) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	DB.ExecContext(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, userID)
}

// ErrEmailTaken: já existe conta com o e-mail
var ErrEmailTaken = errors.New("e-mail já cadastrado")

// CreateUser cria uma conta de usuário comum
func CreateUser(ctx context.Context, email, passwordHash, displayName string) (models.User, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	u := models.User{Email: strings.ToLower(strings.TrimSpace(email)), Role: models.RoleUser, DisplayName: strings.TrimSpace(displayName)}
	err := DB.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, role, display_name)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`, u.Email, passwordHash, u.Role, u.DisplayName).Scan(&u.ID, &u.CreatedAt)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return u, ErrEmailTaken
	}
	return u, err
}

// GetUserByID devolve a conta (ErrNotFound se não existe)
func GetUserByID(ctx context.Context, id int64) (models.User, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var u models.User
	err := DB.QueryRowContext(ctx,
		`SELECT id, email, password_hash, role, display_name, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.DisplayName, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// DeleteUser apaga a conta e o que ela segue; os aparelhos logados nela voltam a convidado
func DeleteUser(ctx context.Context, id int64) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

// AttachDeviceToUser liga o aparelho à conta e passa para ela o que o aparelho seguia como
// convidado (o que a conta já seguia fica como está). Aparelho que ainda não registrou o push
// não tem linha em push_devices: ele é ligado quando registrar (RegisterPushDevice*)
func AttachDeviceToUser(ctx context.Context, deviceID string, userID int64) error {
	ctx, cancel := withTxTimeout(ctx)
	defer cancel()

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, t := range []struct{ table, target string }{
		{"match_subscriptions", "match_id"}, {"team_subscriptions", "team_api_id"}, {"league_subscriptions", "league"},
	} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
            INSERT INTO %[1]s (user_id, %[2]s, kinds, created_at)
            SELECT $2, %[2]s, kinds, created_at FROM %[1]s WHERE device_id = $1
            ON CONFLICT (user_id, %[2]s) DO NOTHING`, t.table, t.target), deviceID, userID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE device_id = $1`, t.table), deviceID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE push_devices SET user_id = $2 WHERE device_id = $1`, deviceID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// DetachDevice desliga o aparelho da conta (saiu da conta): ele para de receber os avisos dela
func DetachDevice(ctx context.Context, deviceID string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `UPDATE push_devices SET user_id = NULL WHERE device_id = $1`, deviceID)
	return err
}

// SetDeviceUser liga um aparelho recém-registrado à conta em que ele está logado
func SetDeviceUser(ctx context.Context, deviceID string, userID int64) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `UPDATE push_devices SET user_id = $2 WHERE device_id = $1`, deviceID, userID)
	return err
}
