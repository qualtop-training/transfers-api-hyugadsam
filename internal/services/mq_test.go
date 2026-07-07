package services

import (
	"context"
	"errors"
	"testing"
	"transfers-api/internal/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMqService_Read(t *testing.T) {
	type testCase struct {
		name      string
		mockSetup func(pub *mocks.TransfersPublisherMock)
		wantMsg   string
		wantErr   bool
	}

	tests := []testCase{
		{
			name: "success",
			mockSetup: func(pub *mocks.TransfersPublisherMock) {
				pub.On("Read").Return("transfer-event", nil)
			},
			wantMsg: "transfer-event",
			wantErr: false,
		},
		{
			name: "publisher_error",
			mockSetup: func(pub *mocks.TransfersPublisherMock) {
				pub.On("Read").Return("", errors.New("queue empty"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub := new(mocks.TransfersPublisherMock)
			tt.mockSetup(pub)

			svc := NewMqService(pub)
			msg, err := svc.Read(context.Background())

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantMsg, msg)
			}

			pub.AssertExpectations(t)
		})
	}
}
