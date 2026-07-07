package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"transfers-api/internal/handlers"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCORSRouter() *gin.Engine {
	r := gin.New()
	r.Use(handlers.AllowCORS)
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestAllowCORS_SetsHeaders(t *testing.T) {
	r := newCORSRouter()
	req, err := http.NewRequest(http.MethodGet, "/ping", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
}

func TestAllowCORS_PreflightReturns204(t *testing.T) {
	r := newCORSRouter()
	req, err := http.NewRequest(http.MethodOptions, "/ping", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}
