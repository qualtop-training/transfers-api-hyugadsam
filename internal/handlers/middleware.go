package handlers

import (
	"net/http"
	"strings"
	"transfers-api/internal/auth"

	"github.com/gin-gonic/gin"
)

// TokenValidator is the minimal interface the middleware needs — just validation,
// decoupled from the full TokenManager so callers don't need to supply generation methods.
//
//go:generate mockery --name TokenValidator --structname TokenValidatorMock --filename token_validator_mock.go --output mocks --outpkg mocks
type TokenValidator interface {
	ValidateToken(token string) (*auth.Claims, error)
}

// NewJWTMiddleware returns a Gin middleware that enforces a valid Bearer token.
// On success it injects "user_id" and "role" into the Gin context.
// On failure it aborts with 401 and a JSON error body.
func NewJWTMiddleware(tv TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header is required"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header must be Bearer <token>"})
			return
		}

		tokenStr := strings.TrimSpace(parts[1])
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token must not be empty"})
			return
		}

		claims, err := tv.ValidateToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}
