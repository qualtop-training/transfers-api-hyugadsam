package transport_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"transfers-api/internal/auth"
	"transfers-api/internal/handlers"
	"transfers-api/internal/transport"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

// ─── minimal stub handlers ────────────────────────────────────────────────────

type stubTransfersHandler struct{}

func (h *stubTransfersHandler) Create(c *gin.Context)      { c.Status(http.StatusOK) }
func (h *stubTransfersHandler) GetByID(c *gin.Context)     { c.Status(http.StatusOK) }
func (h *stubTransfersHandler) Update(c *gin.Context)      { c.Status(http.StatusOK) }
func (h *stubTransfersHandler) Delete(c *gin.Context)      { c.Status(http.StatusOK) }
func (h *stubTransfersHandler) GetByUserID(c *gin.Context) { c.Status(http.StatusOK) }

type stubMqHandler struct{}

func (h *stubMqHandler) Read(c *gin.Context) { c.Status(http.StatusOK) }

type stubAuthHandler struct{}

func (h *stubAuthHandler) Register(c *gin.Context)     { c.Status(http.StatusCreated) }
func (h *stubAuthHandler) Login(c *gin.Context)        { c.Status(http.StatusOK) }
func (h *stubAuthHandler) RefreshToken(c *gin.Context) { c.Status(http.StatusOK) }
func (h *stubAuthHandler) Logout(c *gin.Context)       { c.Status(http.StatusOK) }

// ─── TokenValidator mocks ─────────────────────────────────────────────────────

type allowAllValidator struct{}

func (v *allowAllValidator) ValidateToken(_ string) (*auth.Claims, error) {
	return &auth.Claims{UserID: "u-1", Role: "user"}, nil
}

type denyAllValidator struct{}

func (v *denyAllValidator) ValidateToken(_ string) (*auth.Claims, error) {
	return nil, errors.New("invalid token")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func newTestServer(tv handlers.TokenValidator) *transport.HTTPServer {
	s := transport.NewHTTPServer(
		&stubTransfersHandler{},
		&stubMqHandler{},
		&stubAuthHandler{},
		tv,
	)
	s.MapRoutes()
	return s
}

func do(t *testing.T, server *transport.HTTPServer, method, path, bearer string) int {
	t.Helper()
	req, err := http.NewRequest(method, path, nil)
	require.NoError(t, err)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	return w.Code
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestHTTPServer_PublicAuthRoutes_DoNotRequireToken(t *testing.T) {
	// Use deny-all validator: if any of these routes run the middleware, they'd return 401.
	server := newTestServer(&denyAllValidator{})

	assert.Equal(t, http.StatusCreated, do(t, server, http.MethodPost, "/auth/register", ""))
	assert.Equal(t, http.StatusOK, do(t, server, http.MethodPost, "/auth/login", ""))
	assert.Equal(t, http.StatusOK, do(t, server, http.MethodPost, "/auth/refresh", ""))
}

func TestHTTPServer_LogoutRequiresToken(t *testing.T) {
	server := newTestServer(&denyAllValidator{})
	assert.Equal(t, http.StatusUnauthorized, do(t, server, http.MethodPost, "/auth/logout", ""))
}

func TestHTTPServer_TransferRoutesRequireToken(t *testing.T) {
	server := newTestServer(&denyAllValidator{})

	routes := []struct{ method, path string }{
		{http.MethodPost, "/transfers"},
		{http.MethodGet, "/transfers"},
		{http.MethodGet, "/transfers/abc"},
		{http.MethodPut, "/transfers/abc"},
		{http.MethodDelete, "/transfers/abc"},
	}
	for _, r := range routes {
		assert.Equal(t, http.StatusUnauthorized, do(t, server, r.method, r.path, ""),
			"%s %s should require a token", r.method, r.path)
	}
}

func TestHTTPServer_MqRouteRequiresToken(t *testing.T) {
	server := newTestServer(&denyAllValidator{})
	assert.Equal(t, http.StatusUnauthorized, do(t, server, http.MethodGet, "/mq/", ""))
}

func TestHTTPServer_TransferRoutesPassWithValidToken(t *testing.T) {
	server := newTestServer(&allowAllValidator{})

	routes := []struct{ method, path string }{
		{http.MethodPost, "/transfers"},
		{http.MethodGet, "/transfers"},
		{http.MethodGet, "/transfers/abc"},
		{http.MethodPut, "/transfers/abc"},
		{http.MethodDelete, "/transfers/abc"},
	}
	for _, r := range routes {
		assert.Equal(t, http.StatusOK, do(t, server, r.method, r.path, "valid-token"),
			"%s %s should succeed with a valid token", r.method, r.path)
	}
}

// keep unused var guard out — imports are clean
