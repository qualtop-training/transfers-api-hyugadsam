package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"transfers-api/internal/handlers"
	"transfers-api/internal/handlers/mocks"
	"transfers-api/internal/known_errors"
	"transfers-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ─── router helpers ──────────────────────────────────────────────────────────

func newAuthRouter(svc handlers.AuthService) *gin.Engine {
	r := gin.New()
	h := handlers.NewAuthHandler(svc)
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)
	r.POST("/auth/refresh", h.RefreshToken)
	// Simulate JWT middleware having injected user_id into context.
	r.POST("/auth/logout", func(c *gin.Context) { c.Set("user_id", "u-1"); c.Next() }, h.Logout)
	return r
}

// newAuthRouterNoUserID builds a router whose logout route does NOT inject user_id,
// simulating a request that bypasses or fails the JWT middleware.
func newAuthRouterNoUserID(svc handlers.AuthService) *gin.Engine {
	r := gin.New()
	h := handlers.NewAuthHandler(svc)
	r.POST("/auth/logout", h.Logout)
	return r
}

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewBuffer(b)
}

// anyCtx matches any context.Context — the handler passes c.Request.Context(),
// not context.Background(), so we can't match exactly.
var anyCtx = mock.MatchedBy(func(_ context.Context) bool { return true })

// ─── Register ────────────────────────────────────────────────────────────────

func TestAuthHandler_Register(t *testing.T) {
	type testCase struct {
		name       string
		body       any
		mockSetup  func(svc *mocks.AuthServiceMock)
		wantStatus int
		wantBody   string
	}

	tests := []testCase{
		{
			name: "201_success",
			body: map[string]string{"username": "alice", "email": "a@b.com", "password": "pw", "role": "user"},
			mockSetup: func(svc *mocks.AuthServiceMock) {
				svc.On("Register", anyCtx, "alice", "a@b.com", "pw", "user").
					Return(services.RegisterResult{UserID: "u-123"}, nil)
			},
			wantStatus: http.StatusCreated,
			wantBody:   `"user_id":"u-123"`,
		},
		{
			name: "400_bad_request_from_service",
			body: map[string]string{"username": "", "email": "a@b.com", "password": "pw", "role": "user"},
			mockSetup: func(svc *mocks.AuthServiceMock) {
				svc.On("Register", anyCtx, "", "a@b.com", "pw", "user").
					Return(services.RegisterResult{}, fmt.Errorf("bad: %w", known_errors.ErrBadRequest))
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "409_duplicate_email",
			body: map[string]string{"username": "alice", "email": "a@b.com", "password": "pw", "role": "user"},
			mockSetup: func(svc *mocks.AuthServiceMock) {
				svc.On("Register", anyCtx, "alice", "a@b.com", "pw", "user").
					Return(services.RegisterResult{}, fmt.Errorf("dup: %w", known_errors.ErrDuplicated))
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "500_internal_error",
			body: map[string]string{"username": "alice", "email": "a@b.com", "password": "pw", "role": "user"},
			mockSetup: func(svc *mocks.AuthServiceMock) {
				svc.On("Register", anyCtx, "alice", "a@b.com", "pw", "user").
					Return(services.RegisterResult{}, errors.New("db down"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mocks.AuthServiceMock)
			tt.mockSetup(svc)

			w := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodPost, "/auth/register", jsonBody(t, tt.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			newAuthRouter(svc).ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody)
			}
			svc.AssertExpectations(t)
		})
	}
}

// ─── Login ───────────────────────────────────────────────────────────────────

func TestAuthHandler_Login(t *testing.T) {
	type testCase struct {
		name       string
		body       any
		mockSetup  func(svc *mocks.AuthServiceMock)
		wantStatus int
		wantBody   string
	}

	tests := []testCase{
		{
			name: "200_success",
			body: map[string]string{"email": "a@b.com", "password": "pw"},
			mockSetup: func(svc *mocks.AuthServiceMock) {
				svc.On("Login", anyCtx, "a@b.com", "pw").
					Return(services.LoginResult{AccessToken: "acc", RefreshToken: "ref", UserID: "u-1"}, nil)
			},
			wantStatus: http.StatusOK,
			wantBody:   `"access_token":"acc"`,
		},
		{
			name: "401_wrong_credentials",
			body: map[string]string{"email": "a@b.com", "password": "wrong"},
			mockSetup: func(svc *mocks.AuthServiceMock) {
				svc.On("Login", anyCtx, "a@b.com", "wrong").
					Return(services.LoginResult{}, fmt.Errorf("creds: %w", known_errors.ErrUnauthorized))
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "500_internal_error",
			body: map[string]string{"email": "a@b.com", "password": "pw"},
			mockSetup: func(svc *mocks.AuthServiceMock) {
				svc.On("Login", anyCtx, "a@b.com", "pw").
					Return(services.LoginResult{}, errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mocks.AuthServiceMock)
			tt.mockSetup(svc)

			w := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodPost, "/auth/login", jsonBody(t, tt.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			newAuthRouter(svc).ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody)
			}
			svc.AssertExpectations(t)
		})
	}
}

// ─── RefreshToken ─────────────────────────────────────────────────────────────

func TestAuthHandler_RefreshToken(t *testing.T) {
	type testCase struct {
		name       string
		body       any
		mockSetup  func(svc *mocks.AuthServiceMock)
		wantStatus int
		wantBody   string
	}

	tests := []testCase{
		{
			name: "200_success",
			body: map[string]string{"refresh_token": "old-ref"},
			mockSetup: func(svc *mocks.AuthServiceMock) {
				svc.On("RefreshToken", anyCtx, "old-ref").
					Return(services.LoginResult{AccessToken: "new-acc", RefreshToken: "new-ref"}, nil)
			},
			wantStatus: http.StatusOK,
			wantBody:   `"access_token":"new-acc"`,
		},
		{
			name: "401_invalid_token",
			body: map[string]string{"refresh_token": "bad"},
			mockSetup: func(svc *mocks.AuthServiceMock) {
				svc.On("RefreshToken", anyCtx, "bad").
					Return(services.LoginResult{}, fmt.Errorf("inv: %w", known_errors.ErrUnauthorized))
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mocks.AuthServiceMock)
			tt.mockSetup(svc)

			w := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodPost, "/auth/refresh", jsonBody(t, tt.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			newAuthRouter(svc).ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody)
			}
			svc.AssertExpectations(t)
		})
	}
}

// ─── Logout ──────────────────────────────────────────────────────────────────

func TestAuthHandler_Logout(t *testing.T) {
	type testCase struct {
		name       string
		useRouter  func(svc handlers.AuthService) *gin.Engine
		mockSetup  func(svc *mocks.AuthServiceMock)
		wantStatus int
		wantBody   string
	}

	tests := []testCase{
		{
			name:      "200_success",
			useRouter: newAuthRouter, // injects user_id = "u-1"
			mockSetup: func(svc *mocks.AuthServiceMock) {
				svc.On("Logout", anyCtx, "u-1").Return(nil)
			},
			wantStatus: http.StatusOK,
			wantBody:   `"message":"logged out"`,
		},
		{
			name:       "401_no_user_id_in_context",
			useRouter:  newAuthRouterNoUserID,
			mockSetup:  func(svc *mocks.AuthServiceMock) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:      "500_service_error",
			useRouter: newAuthRouter,
			mockSetup: func(svc *mocks.AuthServiceMock) {
				svc.On("Logout", anyCtx, "u-1").Return(errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mocks.AuthServiceMock)
			tt.mockSetup(svc)

			w := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodPost, "/auth/logout", nil)
			require.NoError(t, err)

			tt.useRouter(svc).ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody)
			}
			svc.AssertExpectations(t)
		})
	}
}
