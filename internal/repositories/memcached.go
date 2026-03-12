package repositories

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bradfitz/gomemcache/memcache"

	"transfers-api/internal/config"
	"transfers-api/internal/enums"
	"transfers-api/internal/known_errors"
	"transfers-api/internal/models"
)

type TransfersMemcacheRepo struct {
	client     *memcache.Client
	ttlSeconds int32
}

type transferCacheDAO struct {
	ID         string  `json:"id"`
	SenderID   string  `json:"sender_id"`
	ReceiverID string  `json:"receiver_id"`
	Currency   string  `json:"currency"`
	Amount     float64 `json:"amount"`
	State      string  `json:"state"`
}

func NewTransfersMemcachedRepository(cfg config.Memcached) *TransfersMemcacheRepo {

	address := fmt.Sprintf("%s:%d", cfg.Hostname, cfg.Port)
	client := memcache.New(address)
	return &TransfersMemcacheRepo{
		client:     client,
		ttlSeconds: int32(cfg.TTLSeconds),
	}
}

func (r *TransfersMemcacheRepo) Create(ctx context.Context, transfer models.Transfer) (string, error) {

	if transfer.ID == "" {
		return "", fmt.Errorf("transfer ID required for cache create")
	}

	dao := transferCacheDAO{
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

	item := &memcache.Item{
		Key:        transfer.ID,
		Value:      data,
		Expiration: r.ttlSeconds,
	}

	err = r.client.Set(item)
	if err != nil {
		return "", fmt.Errorf("error saving transfer in cache: %w", err)
	}

	return transfer.ID, nil
}

func (r *TransfersMemcacheRepo) GetByID(ctx context.Context, id string) (models.Transfer, error) {

	item, err := r.client.Get(id)
	if err != nil {

		if err == memcache.ErrCacheMiss {
			return models.Transfer{}, fmt.Errorf("transfer not found: %w", known_errors.ErrNotFound)
		}

		return models.Transfer{}, fmt.Errorf("error getting transfer from cache: %w", err)
	}

	var dao transferCacheDAO

	err = json.Unmarshal(item.Value, &dao)
	if err != nil {
		return models.Transfer{}, fmt.Errorf("error unmarshaling cached transfer: %w", err)
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

func (r *TransfersMemcacheRepo) Update(ctx context.Context, transfer models.Transfer) error {

	item, err := r.client.Get(transfer.ID)
	if err != nil {

		if err == memcache.ErrCacheMiss {
			return fmt.Errorf("transfer not found: %w", known_errors.ErrNotFound)
		}

		return fmt.Errorf("error retrieving transfer for update: %w", err)
	}

	var dao transferCacheDAO

	err = json.Unmarshal(item.Value, &dao)
	if err != nil {
		return fmt.Errorf("error unmarshaling cached transfer: %w", err)
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

	newItem := &memcache.Item{
		Key:        transfer.ID,
		Value:      data,
		Expiration: r.ttlSeconds,
	}

	err = r.client.Set(newItem)
	if err != nil {
		return fmt.Errorf("error updating transfer in cache: %w", err)
	}

	return nil
}

func (r *TransfersMemcacheRepo) Delete(ctx context.Context, id string) error {

	err := r.client.Delete(id)

	if err != nil {

		if err == memcache.ErrCacheMiss {
			return fmt.Errorf("transfer not found: %w", known_errors.ErrNotFound)
		}

		return fmt.Errorf("error deleting transfer from cache: %w", err)
	}

	return nil
}
