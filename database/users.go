package database

import (
	"App-Futebol/models"
	"strings"
)

func GetUserByEmail(email string) (models.User, error) {
	var u models.User
	err := DB.QueryRow(
		`SELECT id, email, password_hash, role FROM users WHERE email = $1`,
		strings.ToLower(strings.TrimSpace(email)),
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role)
	return u, err
}

// UpsertUser cria o usuário ou, se o e-mail já existir, troca a senha e o papel
func UpsertUser(email string, passwordHash string, role string) (int64, error) {
	var id int64
	err := DB.QueryRow(`
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

func TouchUserLogin(userID int64) {
	DB.Exec(`UPDATE users SET last_login_at = now() WHERE id = $1`, userID)
}
