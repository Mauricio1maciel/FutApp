package models

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
	RoleGuest = "guest" // token de convidado (device_id), sem conta
)

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Role         string
}
