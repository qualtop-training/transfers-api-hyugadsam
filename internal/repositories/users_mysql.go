package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"transfers-api/internal/enums"
	"transfers-api/internal/known_errors"
	"transfers-api/internal/models"
	"transfers-api/internal/services"

	_ "github.com/go-sql-driver/mysql"
)

/*
DDL reference — run once against your MySQL database before starting the service:

CREATE TABLE IF NOT EXISTS users (
    id           INT AUTO_INCREMENT PRIMARY KEY,
    username     VARCHAR(100) NOT NULL,
    email        VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role         VARCHAR(50)  NOT NULL,
    created_at   DATETIME     NOT NULL
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         INT AUTO_INCREMENT PRIMARY KEY,
    user_id    INT          NOT NULL,
    token_hash TEXT         NOT NULL,
    expires_at DATETIME     NOT NULL,
    created_at DATETIME     NOT NULL
);
*/

// compile-time assertion that UsersMySQLRepo satisfies UserRepository.
var _ services.UserRepository = &UsersMySQLRepo{}

// UsersMySQLRepo implements services.UserRepository on top of a MySQL database.
type UsersMySQLRepo struct {
	db *sql.DB
}

// NewUsersMySQLRepository returns a UsersMySQLRepo that reuses an existing *sql.DB connection.
// Pass the same db obtained from NewTransfersMySQLRepository when both repos share a DB.
func NewUsersMySQLRepository(db *sql.DB) *UsersMySQLRepo {
	return &UsersMySQLRepo{db: db}
}

// --- User operations ---

func (r *UsersMySQLRepo) CreateUser(ctx context.Context, user models.User) (string, error) {
	query := `
		INSERT INTO users (username, email, password_hash, role, created_at)
		VALUES (?, ?, ?, ?, ?)`

	res, err := r.db.ExecContext(ctx, query,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.Role.String(),
		user.CreatedAt,
	)
	if err != nil {
		if isDuplicateEntryError(err) {
			return "", fmt.Errorf("user with email %s already exists: %w", user.Email, known_errors.ErrDuplicated)
		}
		return "", fmt.Errorf("inserting user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return "", fmt.Errorf("retrieving inserted user id: %w", err)
	}
	return fmt.Sprintf("%d", id), nil
}

func (r *UsersMySQLRepo) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	query := `
		SELECT id, username, email, password_hash, role, created_at
		FROM users WHERE email = ?`

	return r.scanUser(r.db.QueryRowContext(ctx, query, email))
}

func (r *UsersMySQLRepo) GetUserByID(ctx context.Context, id string) (models.User, error) {
	query := `
		SELECT id, username, email, password_hash, role, created_at
		FROM users WHERE id = ?`

	return r.scanUser(r.db.QueryRowContext(ctx, query, id))
}

// scanUser decodes a single row into a models.User.
func (r *UsersMySQLRepo) scanUser(row *sql.Row) (models.User, error) {
	var (
		id           int64
		username     string
		email        string
		passwordHash string
		role         string
		createdAt    time.Time
	)
	err := row.Scan(&id, &username, &email, &passwordHash, &role, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, fmt.Errorf("user not found: %w", known_errors.ErrNotFound)
		}
		return models.User{}, fmt.Errorf("scanning user row: %w", err)
	}
	return models.User{
		ID:           fmt.Sprintf("%d", id),
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         enums.ParseRole(role),
		CreatedAt:    createdAt,
	}, nil
}

// --- Refresh token operations ---

func (r *UsersMySQLRepo) CreateRefreshToken(ctx context.Context, token models.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting refresh token: %w", err)
	}
	return nil
}

func (r *UsersMySQLRepo) GetRefreshTokenByUserID(ctx context.Context, userID string) (models.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM refresh_tokens WHERE user_id = ?
		ORDER BY created_at DESC LIMIT 1`

	var (
		id        int64
		uid       string
		tokenHash string
		expiresAt time.Time
		createdAt time.Time
	)
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&id, &uid, &tokenHash, &expiresAt, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.RefreshToken{}, fmt.Errorf("refresh token not found: %w", known_errors.ErrNotFound)
		}
		return models.RefreshToken{}, fmt.Errorf("querying refresh token: %w", err)
	}
	return models.RefreshToken{
		ID:        fmt.Sprintf("%d", id),
		UserID:    uid,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		CreatedAt: createdAt,
	}, nil
}

func (r *UsersMySQLRepo) DeleteRefreshTokenByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM refresh_tokens WHERE user_id = ?`

	res, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("deleting refresh token: %w", err)
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return fmt.Errorf("no active refresh token for user %s: %w", userID, known_errors.ErrNotFound)
	}
	return nil
}

// isDuplicateEntryError detects MySQL duplicate-key errors (error 1062).
func isDuplicateEntryError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Duplicate entry") || strings.Contains(msg, "1062")
}
