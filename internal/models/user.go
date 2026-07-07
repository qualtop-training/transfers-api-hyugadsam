package models

import (
	"time"
	"transfers-api/internal/enums"
)

type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	Role         enums.Role
	CreatedAt    time.Time
}
