package services_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
	"transfers-api/internal/enums"
	"transfers-api/internal/known_errors"
	"transfers-api/internal/models"
	"transfers-api/internal/services"
	"transfers-api/internal/services/mocks"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────

func newAuthService(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock, tm *mocks.TokenManagerMock) *services.AuthService {
	return services.NewAuthService(repo, hasher, tm, 7)
}

var (
	ctx      = context.Background()
	testUser = models.User{
		ID:           "user-001",
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hashed",
		Role:         enums.RoleUser,
		CreatedAt:    time.Now().UTC(),
	}
)

// ─────────────────────────────────────────────
// Register
// ─────────────────────────────────────────────

func TestAuthService_Register(t *testing.T) {
	type testCase struct {
		name        string
		username    string
		email       string
		password    string
		role        string
		mockSetup   func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock)
		wantUserID  string
		wantErrWith error
	}

	tests := []testCase{
		{
			name:     "success",
			username: "alice",
			email:    "alice@example.com",
			password: "secret",
			role:     "user",
			mockSetup: func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock) {
				hasher.On("Hash", "secret").Return("hashed_pw", nil)
				repo.On("CreateUser", ctx, mock.MatchedBy(func(u models.User) bool {
					return u.Username == "alice" && u.Email == "alice@example.com" && u.PasswordHash == "hashed_pw"
				})).Return("user-001", nil)
			},
			wantUserID: "user-001",
		},
		{
			name:        "empty_username",
			username:    "",
			email:       "a@b.com",
			password:    "pw",
			role:        "user",
			mockSetup:   func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock) {},
			wantErrWith: known_errors.ErrBadRequest,
		},
		{
			name:        "empty_email",
			username:    "alice",
			email:       "",
			password:    "pw",
			role:        "user",
			mockSetup:   func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock) {},
			wantErrWith: known_errors.ErrBadRequest,
		},
		{
			name:        "empty_password",
			username:    "alice",
			email:       "a@b.com",
			password:    "",
			role:        "user",
			mockSetup:   func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock) {},
			wantErrWith: known_errors.ErrBadRequest,
		},
		{
			name:        "invalid_role",
			username:    "alice",
			email:       "a@b.com",
			password:    "pw",
			role:        "superuser",
			mockSetup:   func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock) {},
			wantErrWith: known_errors.ErrBadRequest,
		},
		{
			name:     "email_duplicated",
			username: "alice",
			email:    "alice@example.com",
			password: "pw",
			role:     "user",
			mockSetup: func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock) {
				hasher.On("Hash", "pw").Return("h", nil)
				repo.On("CreateUser", ctx, mock.Anything).Return("", fmt.Errorf("dup: %w", known_errors.ErrDuplicated))
			},
			wantErrWith: known_errors.ErrDuplicated,
		},
		{
			name:     "repo_error",
			username: "alice",
			email:    "alice@example.com",
			password: "pw",
			role:     "user",
			mockSetup: func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock) {
				hasher.On("Hash", "pw").Return("h", nil)
				repo.On("CreateUser", ctx, mock.Anything).Return("", errors.New("db down"))
			},
			wantErrWith: nil, // generic error, not a sentinel
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mocks.UserRepositoryMock)
			hasher := new(mocks.PasswordHasherMock)
			tm := new(mocks.TokenManagerMock)
			tt.mockSetup(repo, hasher)

			svc := newAuthService(repo, hasher, tm)
			result, err := svc.Register(ctx, tt.username, tt.email, tt.password, tt.role)

			if tt.wantUserID != "" {
				require.NoError(t, err)
				assert.Equal(t, tt.wantUserID, result.UserID)
			} else {
				require.Error(t, err)
				if tt.wantErrWith != nil {
					assert.ErrorIs(t, err, tt.wantErrWith)
				}
			}

			repo.AssertExpectations(t)
			hasher.AssertExpectations(t)
			tm.AssertExpectations(t)
		})
	}
}

// ─────────────────────────────────────────────
// Login
// ─────────────────────────────────────────────

func TestAuthService_Login(t *testing.T) {
	type testCase struct {
		name        string
		email       string
		password    string
		mockSetup   func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock, tm *mocks.TokenManagerMock)
		wantErr     bool
		wantErrWith error
	}

	tests := []testCase{
		{
			name:     "success",
			email:    testUser.Email,
			password: "correct",
			mockSetup: func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock, tm *mocks.TokenManagerMock) {
				repo.On("GetUserByEmail", ctx, testUser.Email).Return(testUser, nil)
				hasher.On("Verify", "correct", testUser.PasswordHash).Return(true, nil)
				// Delete old token — returns not-found, which is silently ignored.
				repo.On("DeleteRefreshTokenByUserID", ctx, testUser.ID).Return(fmt.Errorf("nf: %w", known_errors.ErrNotFound))
				tm.On("GenerateAccessToken", testUser.ID, "user").Return("access-tok", nil)
				tm.On("GenerateRefreshToken", testUser.ID).Return("refresh-tok", nil)
				repo.On("CreateRefreshToken", ctx, mock.MatchedBy(func(rt models.RefreshToken) bool {
					return rt.UserID == testUser.ID && rt.TokenHash == services.HashToken("refresh-tok")
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "user_not_found",
			email:    "ghost@example.com",
			password: "pw",
			mockSetup: func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock, tm *mocks.TokenManagerMock) {
				repo.On("GetUserByEmail", ctx, "ghost@example.com").Return(models.User{}, fmt.Errorf("nf: %w", known_errors.ErrNotFound))
			},
			wantErr:     true,
			wantErrWith: known_errors.ErrUnauthorized,
		},
		{
			name:     "wrong_password",
			email:    testUser.Email,
			password: "wrong",
			mockSetup: func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock, tm *mocks.TokenManagerMock) {
				repo.On("GetUserByEmail", ctx, testUser.Email).Return(testUser, nil)
				hasher.On("Verify", "wrong", testUser.PasswordHash).Return(false, nil)
			},
			wantErr:     true,
			wantErrWith: known_errors.ErrUnauthorized,
		},
		{
			name:     "error_saving_refresh_token",
			email:    testUser.Email,
			password: "correct",
			mockSetup: func(repo *mocks.UserRepositoryMock, hasher *mocks.PasswordHasherMock, tm *mocks.TokenManagerMock) {
				repo.On("GetUserByEmail", ctx, testUser.Email).Return(testUser, nil)
				hasher.On("Verify", "correct", testUser.PasswordHash).Return(true, nil)
				repo.On("DeleteRefreshTokenByUserID", ctx, testUser.ID).Return(nil)
				tm.On("GenerateAccessToken", testUser.ID, "user").Return("access-tok", nil)
				tm.On("GenerateRefreshToken", testUser.ID).Return("refresh-tok", nil)
				repo.On("CreateRefreshToken", ctx, mock.Anything).Return(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mocks.UserRepositoryMock)
			hasher := new(mocks.PasswordHasherMock)
			tm := new(mocks.TokenManagerMock)
			tt.mockSetup(repo, hasher, tm)

			svc := newAuthService(repo, hasher, tm)
			result, err := svc.Login(ctx, tt.email, tt.password)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrWith != nil {
					assert.ErrorIs(t, err, tt.wantErrWith)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, "access-tok", result.AccessToken)
				assert.Equal(t, "refresh-tok", result.RefreshToken)
				assert.Equal(t, testUser.ID, result.UserID)
			}

			repo.AssertExpectations(t)
			hasher.AssertExpectations(t)
			tm.AssertExpectations(t)
		})
	}
}

// ─────────────────────────────────────────────
// RefreshToken
// ─────────────────────────────────────────────

func TestAuthService_RefreshToken(t *testing.T) {
	validClaims := &services.Claims{
		UserID: testUser.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   testUser.ID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	storedToken := models.RefreshToken{
		ID:        "tok-1",
		UserID:    testUser.ID,
		TokenHash: services.HashToken("old-refresh-tok"),
		ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now().UTC(),
	}

	type testCase struct {
		name        string
		token       string
		mockSetup   func(repo *mocks.UserRepositoryMock, tm *mocks.TokenManagerMock)
		wantErr     bool
		wantErrWith error
	}

	tests := []testCase{
		{
			name:  "success_with_rotation",
			token: "old-refresh-tok",
			mockSetup: func(repo *mocks.UserRepositoryMock, tm *mocks.TokenManagerMock) {
				tm.On("ValidateToken", "old-refresh-tok").Return(validClaims, nil)
				repo.On("GetRefreshTokenByUserID", ctx, testUser.ID).Return(storedToken, nil)
				repo.On("DeleteRefreshTokenByUserID", ctx, testUser.ID).Return(nil)
				repo.On("GetUserByID", ctx, testUser.ID).Return(testUser, nil)
				tm.On("GenerateAccessToken", testUser.ID, "user").Return("new-access", nil)
				tm.On("GenerateRefreshToken", testUser.ID).Return("new-refresh", nil)
				repo.On("CreateRefreshToken", ctx, mock.MatchedBy(func(rt models.RefreshToken) bool {
					return rt.UserID == testUser.ID && rt.TokenHash == services.HashToken("new-refresh")
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "invalid_jwt",
			token: "bad-token",
			mockSetup: func(repo *mocks.UserRepositoryMock, tm *mocks.TokenManagerMock) {
				tm.On("ValidateToken", "bad-token").Return((*services.Claims)(nil), errors.New("invalid"))
			},
			wantErr:     true,
			wantErrWith: known_errors.ErrUnauthorized,
		},
		{
			name:  "token_not_in_db",
			token: "old-refresh-tok",
			mockSetup: func(repo *mocks.UserRepositoryMock, tm *mocks.TokenManagerMock) {
				tm.On("ValidateToken", "old-refresh-tok").Return(validClaims, nil)
				repo.On("GetRefreshTokenByUserID", ctx, testUser.ID).Return(models.RefreshToken{}, fmt.Errorf("nf: %w", known_errors.ErrNotFound))
			},
			wantErr:     true,
			wantErrWith: known_errors.ErrUnauthorized,
		},
		{
			name:  "hash_mismatch",
			token: "tampered-token",
			mockSetup: func(repo *mocks.UserRepositoryMock, tm *mocks.TokenManagerMock) {
				tm.On("ValidateToken", "tampered-token").Return(validClaims, nil)
				// storedToken has hash of "old-refresh-tok", not "tampered-token"
				repo.On("GetRefreshTokenByUserID", ctx, testUser.ID).Return(storedToken, nil)
			},
			wantErr:     true,
			wantErrWith: known_errors.ErrUnauthorized,
		},
		{
			name:  "expired_stored_token",
			token: "old-refresh-tok",
			mockSetup: func(repo *mocks.UserRepositoryMock, tm *mocks.TokenManagerMock) {
				tm.On("ValidateToken", "old-refresh-tok").Return(validClaims, nil)
				expired := models.RefreshToken{
					ID:        "tok-1",
					UserID:    testUser.ID,
					TokenHash: services.HashToken("old-refresh-tok"),
					ExpiresAt: time.Now().UTC().Add(-1 * time.Second), // already expired
					CreatedAt: time.Now().UTC().Add(-8 * 24 * time.Hour),
				}
				repo.On("GetRefreshTokenByUserID", ctx, testUser.ID).Return(expired, nil)
			},
			wantErr:     true,
			wantErrWith: known_errors.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mocks.UserRepositoryMock)
			hasher := new(mocks.PasswordHasherMock)
			tm := new(mocks.TokenManagerMock)
			tt.mockSetup(repo, tm)

			svc := newAuthService(repo, hasher, tm)
			result, err := svc.RefreshToken(ctx, tt.token)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrWith != nil {
					assert.ErrorIs(t, err, tt.wantErrWith)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, "new-access", result.AccessToken)
				assert.Equal(t, "new-refresh", result.RefreshToken)
			}

			repo.AssertExpectations(t)
			tm.AssertExpectations(t)
		})
	}
}

// ─────────────────────────────────────────────
// Logout
// ─────────────────────────────────────────────

func TestAuthService_Logout(t *testing.T) {
	type testCase struct {
		name        string
		userID      string
		mockSetup   func(repo *mocks.UserRepositoryMock)
		wantErr     bool
		wantErrWith error
	}

	tests := []testCase{
		{
			name:   "success",
			userID: testUser.ID,
			mockSetup: func(repo *mocks.UserRepositoryMock) {
				repo.On("DeleteRefreshTokenByUserID", ctx, testUser.ID).Return(nil)
			},
			wantErr: false,
		},
		{
			name:   "no_active_session",
			userID: testUser.ID,
			mockSetup: func(repo *mocks.UserRepositoryMock) {
				repo.On("DeleteRefreshTokenByUserID", ctx, testUser.ID).Return(fmt.Errorf("nf: %w", known_errors.ErrNotFound))
			},
			wantErr:     true,
			wantErrWith: known_errors.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mocks.UserRepositoryMock)
			hasher := new(mocks.PasswordHasherMock)
			tm := new(mocks.TokenManagerMock)
			tt.mockSetup(repo)

			svc := newAuthService(repo, hasher, tm)
			err := svc.Logout(ctx, tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrWith != nil {
					assert.ErrorIs(t, err, tt.wantErrWith)
				}
			} else {
				require.NoError(t, err)
			}

			repo.AssertExpectations(t)
		})
	}
}

// ─────────────────────────────────────────────────────────────────
// Register — hasher error path
// ─────────────────────────────────────────────────────────────────

func TestAuthService_Register_HasherError(t *testing.T) {
	repo := new(mocks.UserRepositoryMock)
	hasher := new(mocks.PasswordHasherMock)
	tm := new(mocks.TokenManagerMock)

	hasher.On("Hash", "pw").Return("", errors.New("rand error"))

	svc := services.NewAuthService(repo, hasher, tm, 7)
	_, err := svc.Register(context.Background(), "alice", "a@b.com", "pw", "user")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "hashing password")
	repo.AssertExpectations(t)
	hasher.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────
// Login — GenerateAccessToken error path
// ─────────────────────────────────────────────────────────────────

func TestAuthService_Login_GenerateAccessTokenError(t *testing.T) {
	repo := new(mocks.UserRepositoryMock)
	hasher := new(mocks.PasswordHasherMock)
	tm := new(mocks.TokenManagerMock)

	repo.On("GetUserByEmail", context.Background(), testUser.Email).Return(testUser, nil)
	hasher.On("Verify", "pw", testUser.PasswordHash).Return(true, nil)
	repo.On("DeleteRefreshTokenByUserID", context.Background(), testUser.ID).Return(nil)
	tm.On("GenerateAccessToken", testUser.ID, "user").Return("", errors.New("sign error"))

	svc := services.NewAuthService(repo, hasher, tm, 7)
	_, err := svc.Login(context.Background(), testUser.Email, "pw")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "generating access token")
}

// ─────────────────────────────────────────────────────────────────
// Login — GenerateRefreshToken error path
// ─────────────────────────────────────────────────────────────────

func TestAuthService_Login_GenerateRefreshTokenError(t *testing.T) {
	repo := new(mocks.UserRepositoryMock)
	hasher := new(mocks.PasswordHasherMock)
	tm := new(mocks.TokenManagerMock)

	repo.On("GetUserByEmail", context.Background(), testUser.Email).Return(testUser, nil)
	hasher.On("Verify", "pw", testUser.PasswordHash).Return(true, nil)
	repo.On("DeleteRefreshTokenByUserID", context.Background(), testUser.ID).Return(nil)
	tm.On("GenerateAccessToken", testUser.ID, "user").Return("access-tok", nil)
	tm.On("GenerateRefreshToken", testUser.ID).Return("", errors.New("sign error"))

	svc := services.NewAuthService(repo, hasher, tm, 7)
	_, err := svc.Login(context.Background(), testUser.Email, "pw")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "generating refresh token")
}

// ─────────────────────────────────────────────────────────────────
// RefreshToken — DeleteRefreshToken error during rotation
// ─────────────────────────────────────────────────────────────────

func TestAuthService_RefreshToken_RotationDeleteError(t *testing.T) {
	validClaims := &services.Claims{
		UserID: testUser.ID,
	}
	storedToken := models.RefreshToken{
		UserID:    testUser.ID,
		TokenHash: services.HashToken("old-tok"),
		ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour),
	}

	repo := new(mocks.UserRepositoryMock)
	hasher := new(mocks.PasswordHasherMock)
	tm := new(mocks.TokenManagerMock)

	tm.On("ValidateToken", "old-tok").Return(validClaims, nil)
	repo.On("GetRefreshTokenByUserID", context.Background(), testUser.ID).Return(storedToken, nil)
	repo.On("DeleteRefreshTokenByUserID", context.Background(), testUser.ID).Return(errors.New("db error"))

	svc := services.NewAuthService(repo, hasher, tm, 7)
	_, err := svc.RefreshToken(context.Background(), "old-tok")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rotating refresh token")
}
