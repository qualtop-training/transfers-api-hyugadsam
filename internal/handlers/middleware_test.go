package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"transfers-api/internal/auth"
	"transfers-api/internal/handlers"
	"transfers-api/internal/handlers/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newMiddlewareRouter wires the middleware onto GET /protected and a final handler
// that echoes back the injected context values.
func newMiddlewareRouter(tv handlers.TokenValidator) *gin.Engine {
	r := gin.New()
	r.GET("/protected", handlers.NewJWTMiddleware(tv), func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		role, _ := c.Get("role")
		c.JSON(http.StatusOK, gin.H{"user_id": userID, "role": role})
	})
	return r
}

func TestJWTMiddleware(t *testing.T) {
	validClaims := &auth.Claims{UserID: "u-1", Role: "admin"}

	type testCase struct {
		name           string
		authHeader     string
		mockSetup      func(tv *mocks.TokenValidatorMock)
		wantStatus     int
		wantUserID     string
		wantRole       string
		wantErrContain string
	}

	tests := []testCase{
		{
			name:       "valid_token_passes_and_injects_claims",
			authHeader: "Bearer valid.token.here",
			mockSetup: func(tv *mocks.TokenValidatorMock) {
				tv.On("ValidateToken", "valid.token.here").Return(validClaims, nil)
			},
			wantStatus: http.StatusOK,
			wantUserID: "u-1",
			wantRole:   "admin",
		},
		{
			name:           "missing_authorization_header_returns_401",
			authHeader:     "",
			mockSetup:      func(tv *mocks.TokenValidatorMock) {},
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "authorization header is required",
		},
		{
			name:           "malformed_header_no_bearer_prefix_returns_401",
			authHeader:     "Token abc123",
			mockSetup:      func(tv *mocks.TokenValidatorMock) {},
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "must be Bearer",
		},
		{
			name:           "bearer_with_empty_token_returns_401",
			authHeader:     "Bearer   ",
			mockSetup:      func(tv *mocks.TokenValidatorMock) {},
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "must not be empty",
		},
		{
			name:       "invalid_token_returns_401",
			authHeader: "Bearer bad.token",
			mockSetup: func(tv *mocks.TokenValidatorMock) {
				tv.On("ValidateToken", "bad.token").Return((*auth.Claims)(nil), errors.New("signature invalid"))
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "invalid or expired token",
		},
		{
			name:       "expired_token_returns_401",
			authHeader: "Bearer expired.token",
			mockSetup: func(tv *mocks.TokenValidatorMock) {
				tv.On("ValidateToken", "expired.token").Return((*auth.Claims)(nil), errors.New("token is expired"))
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "invalid or expired token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv := new(mocks.TokenValidatorMock)
			tt.mockSetup(tv)

			router := newMiddlewareRouter(tv)

			req, err := http.NewRequest(http.MethodGet, "/protected", nil)
			require.NoError(t, err)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.wantStatus == http.StatusOK {
				body := w.Body.String()
				assert.Contains(t, body, tt.wantUserID)
				assert.Contains(t, body, tt.wantRole)
			}
			if tt.wantErrContain != "" {
				assert.Contains(t, w.Body.String(), tt.wantErrContain)
			}

			tv.AssertExpectations(t)
		})
	}
}
