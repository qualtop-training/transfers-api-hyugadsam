package services

import (
	"fmt"
	"time"
	"transfers-api/internal/auth"
	"transfers-api/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is re-exported from the auth sub-package for backward compatibility.
type Claims = auth.Claims

// TokenManager defines token generation and validation behaviour.
//
//go:generate mockery --name TokenManager --structname TokenManagerMock --filename token_manager_mock.go --output mocks --outpkg mocks
type TokenManager interface {
	GenerateAccessToken(userID, role string) (string, error)
	GenerateRefreshToken(userID string) (string, error)
	ValidateToken(token string) (*Claims, error)
}

type jwtService struct {
	cfg config.AuthConfig
}

// NewJWTService returns a TokenManager backed by HS512-signed JWTs.
func NewJWTService(cfg config.AuthConfig) TokenManager {
	return &jwtService{cfg: cfg}
}

// GenerateAccessToken creates a short-lived HS512 access token.
func (j *jwtService) GenerateAccessToken(userID, role string) (string, error) {
	ttl := time.Duration(j.cfg.AccessTokenTTLHours) * time.Hour
	claims := &Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signed, err := token.SignedString([]byte(j.cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("signing access token: %w", err)
	}
	return signed, nil
}

// GenerateRefreshToken creates a long-lived HS512 refresh token (no role claim).
func (j *jwtService) GenerateRefreshToken(userID string) (string, error) {
	ttl := time.Duration(j.cfg.RefreshTokenTTLDays) * 24 * time.Hour
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signed, err := token.SignedString([]byte(j.cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("signing refresh token: %w", err)
	}
	return signed, nil
}

// ValidateToken parses and validates a JWT, returning its claims.
func (j *jwtService) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(j.cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("validating token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}
	return claims, nil
}
