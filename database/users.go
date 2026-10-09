package database

import (
	"App-Futebol/models"
	"context"
	"strings"
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
