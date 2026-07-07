// Package auth holds shared authentication types used across services, mocks, and handlers.
package auth

import "github.com/golang-jwt/jwt/v5"

// Claims is the JWT claims payload used by JWTService and middleware.
type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}
