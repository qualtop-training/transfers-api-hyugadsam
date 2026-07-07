package handlers

import (
	"context"
	"errors"
	"net/http"
	"transfers-api/internal/known_errors"
	"transfers-api/internal/services"

	"github.com/gin-gonic/gin"
)

// AuthService is the interface consumed by AuthHandler.
//
//go:generate mockery --name AuthService --structname AuthServiceMock --filename auth_service_mock.go --output mocks --outpkg mocks
type AuthService interface {
	Register(ctx context.Context, username, email, password, role string) (services.RegisterResult, error)
	Login(ctx context.Context, email, password string) (services.LoginResult, error)
	RefreshToken(ctx context.Context, refreshToken string) (services.LoginResult, error)
	Logout(ctx context.Context, userID string) error
}

// AuthHandler exposes authentication endpoints over HTTP.
type AuthHandler struct {
	svc AuthService
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(svc AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// ─── request / response types ───────────────────────────────────────────────

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// ─── handlers ───────────────────────────────────────────────────────────────

// Register godoc
// POST /auth/register
// Body: {username, email, password, role}
// 201 {user_id} | 400 | 409 | 500
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.svc.Register(c.Request.Context(), req.Username, req.Email, req.Password, req.Role)
	if err != nil {
		c.JSON(authErrStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user_id": result.UserID})
}

// Login godoc
// POST /auth/login
// Body: {email, password}
// 200 {access_token, refresh_token} | 401 | 500
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(authErrStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
	})
}

// RefreshToken godoc
// POST /auth/refresh
// Body: {refresh_token}
// 200 {access_token, refresh_token} | 401 | 500
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.svc.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(authErrStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
	})
}

// Logout godoc
// POST /auth/logout  (requires JWT middleware — user_id is injected into context)
// 200 {message} | 401 | 500
func (h *AuthHandler) Logout(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found in context"})
		return
	}

	uid, ok := userID.(string)
	if !ok || uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id in context"})
		return
	}

	if err := h.svc.Logout(c.Request.Context(), uid); err != nil {
		c.JSON(authErrStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// authErrStatus maps domain sentinel errors to HTTP status codes.
func authErrStatus(err error) int {
	switch {
	case errors.Is(err, known_errors.ErrBadRequest):
		return http.StatusBadRequest
	case errors.Is(err, known_errors.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, known_errors.ErrDuplicated):
		return http.StatusConflict
	case errors.Is(err, known_errors.ErrNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
