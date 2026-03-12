package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"transfers-api/internal/config"
	"transfers-api/internal/enums"
	"transfers-api/internal/known_errors"
	"transfers-api/internal/models"

	"github.com/karlseguin/ccache/v3"
)

type TransfersLocalCacheRepo struct {
	client     *ccache.Cache[string]
	ttlSeconds int32
}

type transferLocalCacheDAO struct {
	ID         string  `json:"id"`
	SenderID   string  `json:"sender_id"`
	ReceiverID string  `json:"receiver_id"`
	Currency   string  `json:"currency"`
	Amount     float64 `json:"amount"`
	State      string  `json:"state"`
}

func NewTransfersLocalCacheRepository(cfg config.LocalCache) *TransfersLocalCacheRepo {

	client := ccache.New(ccache.Configure[string]())
	return &TransfersLocalCacheRepo{
		client:     client,
		ttlSeconds: int32(cfg.TTLSeconds),
	}
}

func (r *TransfersLocalCacheRepo) Create(ctx context.Context, transfer models.Transfer) (string, error) {

	if transfer.ID == "" {
		return "", fmt.Errorf("transfer ID required for cache create")
	}

	dao := transferLocalCacheDAO{
		ID:         transfer.ID,
		SenderID:   transfer.SenderID,
		ReceiverID: transfer.ReceiverID,
		Currency:   transfer.Currency.String(),
		Amount:     transfer.Amount,
		State:      transfer.State,
	}

	data, err := json.Marshal(dao)
	if err != nil {
		return "", fmt.Errorf("error marshaling transfer: %w", err)
	}

	r.client.Set(getTransferCacheKey(transfer.ID), string(data), time.Duration(time.Second * time.Duration(r.ttlSeconds)))

	return transfer.ID, nil
}

func getTransferCacheKey(ID string) string{
	return fmt.Sprintf("transfer-%s", ID)
}

func (r *TransfersLocalCacheRepo) GetByID(ctx context.Context, id string) (models.Transfer, error) {

	item := r.client.Get(getTransferCacheKey(id))
	var dao transferLocalCacheDAO

	err := json.Unmarshal([]byte(item.Value()), &dao)
	if err != nil {
		return models.Transfer{}, fmt.Errorf("error unmarshaling local cached transfer: %w", err)
	}

	return models.Transfer{
		ID:         dao.ID,
		SenderID:   dao.SenderID,
		ReceiverID: dao.ReceiverID,
		Currency:   enums.ParseCurrency(dao.Currency),
		Amount:     dao.Amount,
		State:      dao.State,
	}, nil
}

func (r *TransfersLocalCacheRepo) Update(ctx context.Context, transfer models.Transfer) error {

	item := r.client.Get(getTransferCacheKey(transfer.ID))
	if item == nil {
		return fmt.Errorf("transfer not found: %w", known_errors.ErrNotFound)
	}

	var dao transferLocalCacheDAO

	err := json.Unmarshal([]byte(item.Value()), &dao)
	if err != nil {
		return fmt.Errorf("error unmarshaling local cached transfer: %w", err)
	}

	if transfer.SenderID != "" {
		dao.SenderID = transfer.SenderID
	}

	if transfer.ReceiverID != "" {
		dao.ReceiverID = transfer.ReceiverID
	}

	if transfer.Currency != enums.CurrencyUnknown {
		dao.Currency = transfer.Currency.String()
	}

	if transfer.Amount != 0 {
		dao.Amount = transfer.Amount
	}

	if transfer.State != "" {
		dao.State = transfer.State
	}

	data, err := json.Marshal(dao)
	if err != nil {
		return fmt.Errorf("error marshaling updated transfer: %w", err)
	}

	r.client.Set(getTransferCacheKey(transfer.ID), string(data), time.Duration(time.Second * time.Duration(r.ttlSeconds)))

	return nil
}

func (r *TransfersLocalCacheRepo) Delete(ctx context.Context, id string) error {

	ok := r.client.Delete(getTransferCacheKey(id))

	if !ok {
		return fmt.Errorf("error deleting transfer from local cache")
	}

	return nil
}