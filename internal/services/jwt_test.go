package services

import (
	"testing"
	"time"
	"transfers-api/internal/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		JWTSecret:           "test-secret-key-for-unit-tests",
		AccessTokenTTLHours: 1,
		RefreshTokenTTLDays: 7,
	}
}

func TestJWTService_GenerateAndValidateAccessToken(t *testing.T) {
	svc := NewJWTService(newTestAuthConfig())

	token, err := svc.GenerateAccessToken("user-123", "admin")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := svc.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "admin", claims.Role)
}

func TestJWTService_GenerateAndValidateRefreshToken(t *testing.T) {
	svc := NewJWTService(newTestAuthConfig())

	token, err := svc.GenerateRefreshToken("user-456")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := svc.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "user-456", claims.UserID)
	// Refresh token has no role claim — empty string is expected.
	assert.Empty(t, claims.Role)
}

func TestJWTService_ClaimsContainCorrectUserIDAndRole(t *testing.T) {
	svc := NewJWTService(newTestAuthConfig())

	type testCase struct {
		name   string
		userID string
		role   string
	}
	tests := []testCase{
		{name: "admin_role", userID: "u1", role: "admin"},
		{name: "user_role", userID: "u2", role: "user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := svc.GenerateAccessToken(tt.userID, tt.role)
			require.NoError(t, err)

			claims, err := svc.ValidateToken(token)
			require.NoError(t, err)
			assert.Equal(t, tt.userID, claims.UserID)
			assert.Equal(t, tt.role, claims.Role)
		})
	}
}

func TestJWTService_ExpiredTokenReturnsError(t *testing.T) {
	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-unit-tests",
		AccessTokenTTLHours: 1,
		RefreshTokenTTLDays: 7,
	}
	svc := NewJWTService(cfg)

	// Craft a token that expired 1 second ago.
	claims := &Claims{
		UserID: "user-789",
		Role:   "user",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-789",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Second)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signed, err := token.SignedString([]byte(cfg.JWTSecret))
	require.NoError(t, err)

	_, err = svc.ValidateToken(signed)
	assert.Error(t, err, "expired token must return an error")
}

func TestJWTService_WrongSecretReturnsError(t *testing.T) {
	svc := NewJWTService(newTestAuthConfig())
	token, err := svc.GenerateAccessToken("user-111", "user")
	require.NoError(t, err)

	// Validate with a different secret.
	wrongCfg := config.AuthConfig{
		JWTSecret:           "completely-different-secret",
		AccessTokenTTLHours: 1,
		RefreshTokenTTLDays: 7,
	}
	wrongSvc := NewJWTService(wrongCfg)
	_, err = wrongSvc.ValidateToken(token)
	assert.Error(t, err, "token signed with a different secret must fail validation")
}
