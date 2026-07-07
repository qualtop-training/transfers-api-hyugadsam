package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"transfers-api/internal/enums"
	"transfers-api/internal/known_errors"
	"transfers-api/internal/models"
)

// UserRepository is the persistence contract for users and refresh tokens.
// Implementations: repositories.UsersMySQLRepo, repositories.UsersMongoDBRepo.
//
//go:generate mockery --name UserRepository --structname UserRepositoryMock --filename user_repository_mock.go --output mocks --outpkg mocks
type UserRepository interface {
	CreateUser(ctx context.Context, user models.User) (string, error)
	GetUserByEmail(ctx context.Context, email string) (models.User, error)
	GetUserByID(ctx context.Context, id string) (models.User, error)
	CreateRefreshToken(ctx context.Context, token models.RefreshToken) error
	GetRefreshTokenByUserID(ctx context.Context, userID string) (models.RefreshToken, error)
	DeleteRefreshTokenByUserID(ctx context.Context, userID string) error
}

// AuthService handles registration, login, token refresh, and logout.
type AuthService struct {
	repo    UserRepository
	hasher  PasswordHasher
	tokens  TokenManager
	refreshTTLDays int
}

// NewAuthService constructs an AuthService with the given dependencies.
func NewAuthService(repo UserRepository, hasher PasswordHasher, tokens TokenManager, refreshTTLDays int) *AuthService {
	return &AuthService{
		repo:           repo,
		hasher:         hasher,
		tokens:         tokens,
		refreshTTLDays: refreshTTLDays,
	}
}

// RegisterResult is returned by Register on success.
type RegisterResult struct {
	UserID string
}

// LoginResult is returned by Login on success.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	UserID       string
}

// Register creates a new user account.
// Returns ErrBadRequest for empty fields, ErrDuplicated if email already exists.
func (s *AuthService) Register(ctx context.Context, username, email, password, role string) (RegisterResult, error) {
	if strings.TrimSpace(username) == "" {
		return RegisterResult{}, fmt.Errorf("username is required: %w", known_errors.ErrBadRequest)
	}
	if strings.TrimSpace(email) == "" {
		return RegisterResult{}, fmt.Errorf("email is required: %w", known_errors.ErrBadRequest)
	}
	if strings.TrimSpace(password) == "" {
		return RegisterResult{}, fmt.Errorf("password is required: %w", known_errors.ErrBadRequest)
	}

	parsedRole := enums.ParseRole(role)
	if parsedRole == enums.RoleUnknown {
		return RegisterResult{}, fmt.Errorf("invalid role %q: %w", role, known_errors.ErrBadRequest)
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("hashing password: %w", err)
	}

	user := models.User{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Role:         parsedRole,
		CreatedAt:    time.Now().UTC(),
	}

	id, err := s.repo.CreateUser(ctx, user)
	if err != nil {
		if errors.Is(err, known_errors.ErrDuplicated) {
			return RegisterResult{}, fmt.Errorf("email already registered: %w", known_errors.ErrDuplicated)
		}
		return RegisterResult{}, fmt.Errorf("creating user: %w", err)
	}

	return RegisterResult{UserID: id}, nil
}

// Login authenticates a user and returns a new access + refresh token pair.
// Returns ErrUnauthorized if the email is not found or password is wrong.
func (s *AuthService) Login(ctx context.Context, email, password string) (LoginResult, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, known_errors.ErrNotFound) {
			return LoginResult{}, fmt.Errorf("invalid credentials: %w", known_errors.ErrUnauthorized)
		}
		return LoginResult{}, fmt.Errorf("fetching user: %w", err)
	}

	ok, err := s.hasher.Verify(password, user.PasswordHash)
	if err != nil {
		return LoginResult{}, fmt.Errorf("verifying password: %w", err)
	}
	if !ok {
		return LoginResult{}, fmt.Errorf("invalid credentials: %w", known_errors.ErrUnauthorized)
	}

	// Remove any existing refresh token (rotation — only one active per user).
	_ = s.repo.DeleteRefreshTokenByUserID(ctx, user.ID) // ignore "not found" here

	accessToken, err := s.tokens.GenerateAccessToken(user.ID, user.Role.String())
	if err != nil {
		return LoginResult{}, fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, err := s.tokens.GenerateRefreshToken(user.ID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("generating refresh token: %w", err)
	}

	tokenRecord := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashToken(refreshToken),
		ExpiresAt: time.Now().UTC().Add(time.Duration(s.refreshTTLDays) * 24 * time.Hour),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.CreateRefreshToken(ctx, tokenRecord); err != nil {
		return LoginResult{}, fmt.Errorf("saving refresh token: %w", err)
	}

	return LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
	}, nil
}

// RefreshToken validates an existing refresh token, rotates it, and returns a new pair.
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (LoginResult, error) {
	claims, err := s.tokens.ValidateToken(refreshToken)
	if err != nil {
		return LoginResult{}, fmt.Errorf("invalid refresh token: %w", known_errors.ErrUnauthorized)
	}

	stored, err := s.repo.GetRefreshTokenByUserID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, known_errors.ErrNotFound) {
			return LoginResult{}, fmt.Errorf("refresh token not found: %w", known_errors.ErrUnauthorized)
		}
		return LoginResult{}, fmt.Errorf("fetching refresh token: %w", err)
	}

	// Verify the hash matches.
	if stored.TokenHash != hashToken(refreshToken) {
		return LoginResult{}, fmt.Errorf("refresh token mismatch: %w", known_errors.ErrUnauthorized)
	}

	// Check expiry (belt-and-suspenders — JWT validates this too).
	if time.Now().UTC().After(stored.ExpiresAt) {
		return LoginResult{}, fmt.Errorf("refresh token expired: %w", known_errors.ErrUnauthorized)
	}

	// Rotate: delete old, issue new.
	if err := s.repo.DeleteRefreshTokenByUserID(ctx, claims.UserID); err != nil {
		return LoginResult{}, fmt.Errorf("rotating refresh token: %w", err)
	}

	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("fetching user for refresh: %w", err)
	}

	newAccess, err := s.tokens.GenerateAccessToken(user.ID, user.Role.String())
	if err != nil {
		return LoginResult{}, fmt.Errorf("generating access token: %w", err)
	}

	newRefresh, err := s.tokens.GenerateRefreshToken(user.ID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("generating refresh token: %w", err)
	}

	newRecord := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashToken(newRefresh),
		ExpiresAt: time.Now().UTC().Add(time.Duration(s.refreshTTLDays) * 24 * time.Hour),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.CreateRefreshToken(ctx, newRecord); err != nil {
		return LoginResult{}, fmt.Errorf("saving new refresh token: %w", err)
	}

	return LoginResult{
		AccessToken:  newAccess,
		RefreshToken: newRefresh,
		UserID:       user.ID,
	}, nil
}

// Logout invalidates the user's active refresh token.
// Returns ErrNotFound (wrapped) if the user has no active session.
func (s *AuthService) Logout(ctx context.Context, userID string) error {
	if err := s.repo.DeleteRefreshTokenByUserID(ctx, userID); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	return nil
}

// HashToken returns the SHA-256 hex digest of a token string.
// Used to store refresh tokens without keeping the raw value in the database.
// Exported so tests can compute expected values.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// hashToken is the internal alias kept for internal usage clarity.
func hashToken(token string) string { return HashToken(token) }
