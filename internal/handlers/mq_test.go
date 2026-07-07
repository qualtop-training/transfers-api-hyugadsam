package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"transfers-api/internal/handlers"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ─── MqService mock ──────────────────────────────────────────────────────────

type mqServiceMock struct{ mock.Mock }

func (m *mqServiceMock) Read(ctx context.Context) (string, error) {
	ret := m.Called(ctx)
	return ret.String(0), ret.Error(1)
}

// ─── router helper ───────────────────────────────────────────────────────────

func newMqRouter(svc handlers.MqService) *gin.Engine {
	r := gin.New()
	h := handlers.NewMqHandler(svc)
	r.GET("/mq/", h.Read)
	return r
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestMqHandler_Read(t *testing.T) {
	anyCtxMq := mock.MatchedBy(func(_ context.Context) bool { return true })

	type testCase struct {
		name       string
		mockSetup  func(svc *mqServiceMock)
		wantStatus int
		wantBody   string
	}

	tests := []testCase{
		{
			name: "200_success",
			mockSetup: func(svc *mqServiceMock) {
				svc.On("Read", anyCtxMq).Return("event-data", nil)
			},
			wantStatus: http.StatusOK,
			wantBody:   "event-data",
		},
		{
			name: "500_service_error",
			mockSetup: func(svc *mqServiceMock) {
				svc.On("Read", anyCtxMq).Return("", errors.New("mq down"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mqServiceMock)
			tt.mockSetup(svc)

			req, err := http.NewRequest(http.MethodGet, "/mq/", nil)
			require.NoError(t, err)
			w := httptest.NewRecorder()

			newMqRouter(svc).ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody)
			}
			svc.AssertExpectations(t)
		})
	}
}
