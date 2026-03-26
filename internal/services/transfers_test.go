package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"transfers-api/internal/config"
	"transfers-api/internal/enums"
	"transfers-api/internal/models"
	"transfers-api/internal/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)


func TestTransferService_GetByID(t *testing.T) {

	businessCfg := config.BusinessConfig{
		TransferMinAmount: 1,
	}

	transfer := models.Transfer{
		ID:        "transfer1234",
		Amount:    100,
		SenderID:  "Sender",
		ReceiverID:"Receiver",
		Currency:  enums.CurrencyARS,
		State:     "OK",
	}

	msgNotFound := "Not Found"

	type testCase struct {
		name        string
		transferID  string
		mockSetup   func(repo, cache, local *mocks.TransfersRepositoryMock, pub *mocks.TransfersPublisherMock)
		expectedErr string
		expectedSource string // "local", "cache", "repo"
	}

	tests := []testCase{
		{
			name:       "success from local cache",
			transferID: transfer.ID,
			mockSetup: func(repo, cache, local *mocks.TransfersRepositoryMock, pub *mocks.TransfersPublisherMock) {
				local.On("GetByID", context.Background(), transfer.ID).Return(transfer, nil)
			},
			expectedErr:   "",
			expectedSource: "local",
		},
		{
			name:       "success from cache",
			transferID: transfer.ID,
			mockSetup: func(repo, cache, local *mocks.TransfersRepositoryMock, pub *mocks.TransfersPublisherMock) {
				// local falla → fallback a cache
				local.On("GetByID", context.Background(), transfer.ID).Return(models.Transfer{}, errors.New(msgNotFound))
				cache.On("GetByID", context.Background(), transfer.ID).Return(transfer, nil)
			},
			expectedErr:   "",
			expectedSource: "cache",
		},
		{
			name:       "success from repo",
			transferID: transfer.ID,
			mockSetup: func(repo, cache, local *mocks.TransfersRepositoryMock, pub *mocks.TransfersPublisherMock) {
				local.On("GetByID", context.Background(), transfer.ID).Return(models.Transfer{}, errors.New(msgNotFound))
				cache.On("GetByID", context.Background(), transfer.ID).Return(models.Transfer{}, errors.New(msgNotFound))
				repo.On("GetByID", context.Background(), transfer.ID).Return(transfer, nil)

				//Any because in case of error, it will be loged only
				cache.On("Create", mock.Anything, mock.Anything).Return(transfer.ID, nil)
				local.On("Create", mock.Anything, mock.Anything).Return(transfer.ID, nil)
			},
			expectedErr:   "",
			expectedSource: "repo",
		},
		{
			name:       "not found anywhere",
			transferID: transfer.ID,
			mockSetup: func(repo, cache, local *mocks.TransfersRepositoryMock, pub *mocks.TransfersPublisherMock) {
				local.On("GetByID", context.Background(), transfer.ID).Return(models.Transfer{}, errors.New(msgNotFound))
				cache.On("GetByID", context.Background(), transfer.ID).Return(models.Transfer{}, errors.New(msgNotFound))
				repo.On("GetByID", context.Background(), transfer.ID).Return(models.Transfer{}, errors.New(msgNotFound))
			},
			expectedErr:   msgNotFound,
			expectedSource: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Crear mocks por test
			repo := new(mocks.TransfersRepositoryMock)
			cache := new(mocks.TransfersRepositoryMock)
			local := new(mocks.TransfersRepositoryMock)
			pub := new(mocks.TransfersPublisherMock)

			// Setup del test case
			tt.mockSetup(repo, cache, local, pub)

			// Crear service
			service := NewTransfersService(businessCfg, repo, cache, local, pub)

			// Ejecutar GetByID
			result, err := service.GetByID(context.Background(), tt.transferID)

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), msgNotFound)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, transfer.ID, result.ID)
			}

			// Validar de que repositorio provino
			switch tt.expectedSource {
			case "local":
				local.AssertCalled(t, "GetByID", context.Background(), transfer.ID)
				cache.AssertNotCalled(t, "GetByID", context.Background(), transfer.ID)
				repo.AssertNotCalled(t, "GetByID", context.Background(), transfer.ID)
			case "cache":
				local.AssertCalled(t, "GetByID", context.Background(), transfer.ID)
				cache.AssertCalled(t, "GetByID", context.Background(), transfer.ID)
				repo.AssertNotCalled(t, "GetByID", context.Background(), transfer.ID)
			case "repo":
				local.AssertCalled(t, "GetByID", context.Background(), transfer.ID)
				cache.AssertCalled(t, "GetByID", context.Background(), transfer.ID)
				repo.AssertCalled(t, "GetByID", context.Background(), transfer.ID)
			default:
				// not found
				local.AssertCalled(t, "GetByID", context.Background(), transfer.ID)
				cache.AssertCalled(t, "GetByID", context.Background(), transfer.ID)
				repo.AssertCalled(t, "GetByID", context.Background(), transfer.ID)
			}

			// Validar mocks en general
			local.AssertExpectations(t)
			cache.AssertExpectations(t)
			repo.AssertExpectations(t)
			pub.AssertExpectations(t)
		})
	}
}

// noopPublisher ignora Publish para tests sin depender de goroutines
type noopPublisher struct{}

func (n *noopPublisher) Publish(operation string, transferID string) error {
	return nil
}

func (n *noopPublisher) Read() (string, error) {
	return "", nil
}

func TestTransfersService_Create(t *testing.T) {
	businessCfg := config.BusinessConfig{
		TransferMinAmount: 1,
	}

	transfer := models.Transfer{
		SenderID:   "Sender",
		ReceiverID: "Receiver",
		Currency:   enums.CurrencyARS,
		Amount:     100,
		State:      "OK",
	}

	type testCase struct {
		name        string
		input       models.Transfer
		mockSetup   func(repo, cache, local *mocks.TransfersRepositoryMock)
		expectedErr string
	}

	tests := []testCase{
		{
			name:  "success",
			input: transfer,
			mockSetup: func(repo, cache, local *mocks.TransfersRepositoryMock) {
				repo.On("Create", context.Background(), transfer).Return("transfer1234", nil)
				cache.On("Create", context.Background(), mock.AnythingOfType("models.Transfer")).Return("transfer1234", nil)
				local.On("Create", context.Background(), mock.AnythingOfType("models.Transfer")).Return("transfer1234", nil)
			},
			expectedErr: "",
		},
		{
			name:  "sender validation error",
			input: models.Transfer{},
			mockSetup: func(repo, cache, local *mocks.TransfersRepositoryMock) {
				// no se llama a repo ni cache
			},
			expectedErr: "sender_id is required",
		},
		//... validaciones otros campos
		{
			name:  "repo error",
			input: transfer,
			mockSetup: func(repo, cache, local *mocks.TransfersRepositoryMock) {
				repo.On("Create", context.Background(), transfer).Return("", errors.New("db error"))
			},
			expectedErr: "error creating transfer in repository: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mocks.TransfersRepositoryMock)
			cache := new(mocks.TransfersRepositoryMock)
			local := new(mocks.TransfersRepositoryMock)

			tt.mockSetup(repo, cache, local)

			service := &TransfersService{
				businessCfg:         businessCfg,
				transfersRepo:       repo,
				transfersCache:      cache,
				transfersCacheLocal: local,
				transfersPublisher:  &noopPublisher{}, // ignoramos Publish
			}

			id, err := service.Create(context.Background(), tt.input)

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "transfer1234", id)
			}

			repo.AssertExpectations(t)
			cache.AssertExpectations(t)
			local.AssertExpectations(t)
		})
	}
}
// ----------------------
// Test Update
// ----------------------
func TestTransfersService_Update(t *testing.T) {
	businessCfg := config.BusinessConfig{
		TransferMinAmount: 1,
	}

	transfer := models.Transfer{
		ID:         "transfer123",
		SenderID:   "Sender",
		ReceiverID: "Receiver",
		Currency:   enums.CurrencyARS,
		Amount:     100,
		State:      "OK",
	}

	type testCase struct {
		name        string
		input       models.Transfer
		mockSetup   func(repo, cache, local *mocks.TransfersRepositoryMock)
		expectedErr string
	}

	tests := []testCase{
		{
			name:  "success",
			input: transfer,
			mockSetup: func(repo, cache, local *mocks.TransfersRepositoryMock) {
				repo.On("Update", context.Background(), transfer).Return(nil)
				cache.On("Create", context.Background(), mock.AnythingOfType("models.Transfer")).Return(transfer.ID, nil)
				local.On("Create", context.Background(), mock.AnythingOfType("models.Transfer")).Return(transfer.ID, nil)
			},
			expectedErr: "",
		},
		{
			name:  "Id validation error",
			input: models.Transfer{ID: ""},
			mockSetup: func(repo, cache, local *mocks.TransfersRepositoryMock) {
				// no se llama a repo ni cache
			},
			expectedErr: "ID is required",
		},
		//... validaciones otros campos
		{
			name:  "repo error",
			input: transfer,
			mockSetup: func(repo, cache, local *mocks.TransfersRepositoryMock) {
				repo.On("Update", context.Background(), transfer).Return(errors.New("db error"))
			},
			expectedErr: "error updating transfer transfer123 in repository: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mocks.TransfersRepositoryMock)
			cache := new(mocks.TransfersRepositoryMock)
			local := new(mocks.TransfersRepositoryMock)

			tt.mockSetup(repo, cache, local)

			service := &TransfersService{
				businessCfg:         businessCfg,
				transfersRepo:       repo,
				transfersCache:      cache,
				transfersCacheLocal: local,
				transfersPublisher:  &noopPublisher{}, // ignoramos Publish
			}

			err := service.Update(context.Background(), tt.input)

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			repo.AssertExpectations(t)
			cache.AssertExpectations(t)
			local.AssertExpectations(t)
		})
	}
}

// ----------------------
// Test Delete
// ----------------------
func TestTransfersService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		repo := &mocks.TransfersRepositoryMock{}
		cache := &mocks.TransfersRepositoryMock{}
		cacheLocal := &mocks.TransfersRepositoryMock{}

		repo.On("Delete", ctx, "transfer123").Return(nil)
		cache.On("Delete", ctx, "transfer123").Return(nil)
		cacheLocal.On("Delete", ctx, "transfer123").Return(nil)

		service := TransfersService{
			transfersRepo:       repo,
			transfersCache:      cache,
			transfersCacheLocal: cacheLocal,
			transfersPublisher:  &noopPublisher{},
		}

		err := service.Delete(ctx, "transfer123")
		assert.NoError(t, err)

		repo.AssertCalled(t, "Delete", ctx, "transfer123")
	})

	t.Run("repo_error", func(t *testing.T) {
		repo := &mocks.TransfersRepositoryMock{}
		cache := &mocks.TransfersRepositoryMock{}
		cacheLocal := &mocks.TransfersRepositoryMock{}

		repo.On("Delete", ctx, "transfer123").Return(fmt.Errorf("repo error"))
		cache.On("Delete", ctx, "transfer123").Return(nil)
		cacheLocal.On("Delete", ctx, "transfer123").Return(nil)

		service := TransfersService{
			transfersRepo:       repo,
			transfersCache:      cache,
			transfersCacheLocal: cacheLocal,
			transfersPublisher:  &noopPublisher{},
		}

		err := service.Delete(ctx, "transfer123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repo error")
	})

	t.Run("validation_empty_id", func(t *testing.T) {
		repo := &mocks.TransfersRepositoryMock{}
		cache := &mocks.TransfersRepositoryMock{}
		cacheLocal := &mocks.TransfersRepositoryMock{}

		// El metodo no falla con id vacio
		repo.On("Delete", ctx, "").Return(nil)
		cache.On("Delete", ctx, "").Return(nil)
		cacheLocal.On("Delete", ctx, "").Return(nil)

		service := TransfersService{
			transfersRepo:       repo,
			transfersCache:      cache,
			transfersCacheLocal: cacheLocal,
			transfersPublisher:  &noopPublisher{},
		}

		err := service.Delete(ctx, "")
		assert.NoError(t, err)
	})
}














