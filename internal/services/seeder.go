package services

import (
	"context"
	"errors"
	"strings"
	"time"
	"transfers-api/internal/enums"
	"transfers-api/internal/known_errors"
	"transfers-api/internal/logging"
	"transfers-api/internal/models"
)

// SeedDefaultAdmin creates the default admin user if it does not already exist.
// It is idempotent: calling it multiple times on the same DB has no effect.
// If username is empty the seed is skipped entirely.
func SeedDefaultAdmin(
	ctx context.Context,
	repo UserRepository,
	hasher PasswordHasher,
	username, email, password string,
) {
	if strings.TrimSpace(username) == "" {
		return
	}

	// Check whether the user already exists.
	_, err := repo.GetUserByEmail(ctx, email)
	if err == nil {
		logging.Logger.Infof("default admin %q already exists — skipping seed", email)
		return
	}
	if !errors.Is(err, known_errors.ErrNotFound) {
		logging.Logger.Warnf("seeder: unexpected error checking admin existence: %v", err)
		return
	}

	hash, err := hasher.Hash(password)
	if err != nil {
		logging.Logger.Errorf("seeder: failed to hash admin password: %v", err)
		return
	}

	user := models.User{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Role:         enums.RoleAdmin,
		CreatedAt:    time.Now().UTC(),
	}

	id, err := repo.CreateUser(ctx, user)
	if err != nil {
		if errors.Is(err, known_errors.ErrDuplicated) {
			logging.Logger.Infof("seeder: admin user already exists (race condition) — skipping")
			return
		}
		logging.Logger.Errorf("seeder: failed to create admin user: %v", err)
		return
	}

	logging.Logger.Infof("seeder: default admin user created (id=%s, email=%s)", id, email)
}
