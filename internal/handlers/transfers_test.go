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
	"transfers-api/internal/enums"
	"transfers-api/internal/handlers"
	"transfers-api/internal/handlers/mocks"
	"transfers-api/internal/known_errors"
	"transfers-api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ─── router helper ───────────────────────────────────────────────────────────

func newTransfersRouter(svc handlers.TransfersService) *gin.Engine {
	r := gin.New()
	h := handlers.NewTransfersHandler(svc)
	r.POST("/transfers", h.Create)
	r.GET("/transfers/:id", h.GetByID)
	r.PUT("/transfers/:id", h.Update)
	r.DELETE("/transfers/:id", h.Delete)
	r.GET("/transfers", h.GetByUserID)
	return r
}

var anyContext = mock.MatchedBy(func(_ context.Context) bool { return true })

// ─── Create ──────────────────────────────────────────────────────────────────

func TestTransfersHandler_Create(t *testing.T) {
	validBody := map[string]interface{}{
		"sender_id": "s1", "receiver_id": "r1",
		"currency": "USD", "amount": 100.0, "state": "pending",
	}
	validTransfer := models.Transfer{
		SenderID: "s1", ReceiverID: "r1",
		Currency: enums.CurrencyUSD, Amount: 100.0, State: "pending",
	}

	type testCase struct {
		name       string
		body       any
		mockSetup  func(svc *mocks.TransfersServiceMock)
		wantStatus int
		wantBody   string
	}

	tests := []testCase{
		{
			name: "201_success",
			body: validBody,
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("Create", anyContext, validTransfer).Return("t-123", nil)
			},
			wantStatus: http.StatusCreated,
			wantBody:   `"id":"t-123"`,
		},
		{
			name:       "400_invalid_currency",
			body:       map[string]interface{}{"sender_id": "s1", "receiver_id": "r1", "currency": "INVALID", "amount": 100.0, "state": "pending"},
			mockSetup:  func(svc *mocks.TransfersServiceMock) {},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid currency",
		},
		{
			name: "400_service_bad_request",
			body: validBody,
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("Create", anyContext, validTransfer).Return("", fmt.Errorf("bad: %w", known_errors.ErrBadRequest))
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "500_service_error",
			body: validBody,
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("Create", anyContext, validTransfer).Return("", errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mocks.TransfersServiceMock)
			tt.mockSetup(svc)

			b, err := json.Marshal(tt.body)
			require.NoError(t, err)
			req, _ := http.NewRequest(http.MethodPost, "/transfers", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			newTransfersRouter(svc).ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody)
			}
			svc.AssertExpectations(t)
		})
	}
}

// ─── GetByID ─────────────────────────────────────────────────────────────────

func TestTransfersHandler_GetByID(t *testing.T) {
	transfer := models.Transfer{
		ID: "t-1", SenderID: "s1", ReceiverID: "r1",
		Currency: enums.CurrencyUSD, Amount: 50.0, State: "done",
	}

	type testCase struct {
		name       string
		id         string
		mockSetup  func(svc *mocks.TransfersServiceMock)
		wantStatus int
		wantBody   string
	}

	tests := []testCase{
		{
			name: "200_success",
			id:   "t-1",
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("GetByID", anyContext, "t-1").Return(transfer, nil)
			},
			wantStatus: http.StatusOK,
			wantBody:   `"id":"t-1"`,
		},
		{
			name: "400_bad_request",
			id:   "bad-id",
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("GetByID", anyContext, "bad-id").Return(models.Transfer{}, fmt.Errorf("bad: %w", known_errors.ErrBadRequest))
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "404_not_found",
			id:   "missing",
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("GetByID", anyContext, "missing").Return(models.Transfer{}, fmt.Errorf("nf: %w", known_errors.ErrNotFound))
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "500_internal_error",
			id:   "t-1",
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("GetByID", anyContext, "t-1").Return(models.Transfer{}, errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mocks.TransfersServiceMock)
			tt.mockSetup(svc)

			req, _ := http.NewRequest(http.MethodGet, "/transfers/"+tt.id, nil)
			w := httptest.NewRecorder()

			newTransfersRouter(svc).ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody)
			}
			svc.AssertExpectations(t)
		})
	}
}

// ─── Update ──────────────────────────────────────────────────────────────────

func TestTransfersHandler_Update(t *testing.T) {
	type testCase struct {
		name       string
		id         string
		body       any
		mockSetup  func(svc *mocks.TransfersServiceMock)
		wantStatus int
	}

	tests := []testCase{
		{
			name: "200_success",
			id:   "t-1",
			body: map[string]interface{}{"sender_id": "s2", "currency": "EUR", "amount": 200.0, "state": "done"},
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("Update", anyContext, mock.MatchedBy(func(t models.Transfer) bool {
					return t.ID == "t-1" && t.SenderID == "s2"
				})).Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "400_invalid_currency",
			id:         "t-1",
			body:       map[string]interface{}{"currency": "BOGUS"},
			mockSetup:  func(svc *mocks.TransfersServiceMock) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "400_service_bad_request",
			id:   "t-1",
			body: map[string]interface{}{"sender_id": "s2"},
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("Update", anyContext, mock.Anything).Return(fmt.Errorf("bad: %w", known_errors.ErrBadRequest))
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "404_not_found",
			id:   "t-1",
			body: map[string]interface{}{"sender_id": "s2"},
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("Update", anyContext, mock.Anything).Return(fmt.Errorf("nf: %w", known_errors.ErrNotFound))
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "500_internal_error",
			id:   "t-1",
			body: map[string]interface{}{"sender_id": "s2"},
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("Update", anyContext, mock.Anything).Return(errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mocks.TransfersServiceMock)
			tt.mockSetup(svc)

			b, err := json.Marshal(tt.body)
			require.NoError(t, err)
			req, _ := http.NewRequest(http.MethodPut, "/transfers/"+tt.id, bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			newTransfersRouter(svc).ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			svc.AssertExpectations(t)
		})
	}
}

// ─── Delete ──────────────────────────────────────────────────────────────────

func TestTransfersHandler_Delete(t *testing.T) {
	type testCase struct {
		name       string
		id         string
		mockSetup  func(svc *mocks.TransfersServiceMock)
		wantStatus int
	}

	tests := []testCase{
		{
			name: "200_success",
			id:   "t-1",
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("Delete", anyContext, "t-1").Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "404_not_found",
			id:   "missing",
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("Delete", anyContext, "missing").Return(fmt.Errorf("nf: %w", known_errors.ErrNotFound))
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "500_internal_error",
			id:   "t-1",
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("Delete", anyContext, "t-1").Return(errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mocks.TransfersServiceMock)
			tt.mockSetup(svc)

			req, _ := http.NewRequest(http.MethodDelete, "/transfers/"+tt.id, nil)
			w := httptest.NewRecorder()

			newTransfersRouter(svc).ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			svc.AssertExpectations(t)
		})
	}
}

// ─── GetByUserID ─────────────────────────────────────────────────────────────

func TestTransfersHandler_GetByUserID(t *testing.T) {
	transfers := []models.Transfer{
		{ID: "t-1", SenderID: "u1", ReceiverID: "r1", Currency: enums.CurrencyUSD, Amount: 10.0, State: "ok"},
		{ID: "t-2", SenderID: "u1", ReceiverID: "r2", Currency: enums.CurrencyEUR, Amount: 20.0, State: "ok"},
	}

	type testCase struct {
		name       string
		senderID   string
		mockSetup  func(svc *mocks.TransfersServiceMock)
		wantStatus int
		wantBody   string
	}

	tests := []testCase{
		{
			name:     "200_success_with_results",
			senderID: "u1",
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("GetByUserID", anyContext, "u1").Return(transfers, nil)
			},
			wantStatus: http.StatusOK,
			wantBody:   `"id":"t-1"`,
		},
		{
			name:     "200_empty_result",
			senderID: "nobody",
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("GetByUserID", anyContext, "nobody").Return([]models.Transfer{}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "400_bad_request",
			senderID: "u1",
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("GetByUserID", anyContext, "u1").Return(nil, fmt.Errorf("bad: %w", known_errors.ErrBadRequest))
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:     "404_not_found",
			senderID: "u1",
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("GetByUserID", anyContext, "u1").Return(nil, fmt.Errorf("nf: %w", known_errors.ErrNotFound))
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:     "500_internal_error",
			senderID: "u1",
			mockSetup: func(svc *mocks.TransfersServiceMock) {
				svc.On("GetByUserID", anyContext, "u1").Return(nil, errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mocks.TransfersServiceMock)
			tt.mockSetup(svc)

			req, _ := http.NewRequest(http.MethodGet, "/transfers?SenderId="+tt.senderID, nil)
			w := httptest.NewRecorder()

			newTransfersRouter(svc).ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody)
			}
			svc.AssertExpectations(t)
		})
	}
}
