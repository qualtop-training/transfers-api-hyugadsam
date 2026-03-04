package repositories

import (
	"context"
	"fmt"
	"transfers-api/internal/config"
	"transfers-api/internal/enums"
	"transfers-api/internal/models"
)

type TransfersMysqlRepo struct {
	collection string
}

type transferMysqlDAO struct {
	ID         int64			  `bson:"_id,omitempty"`
	SenderID   string             `bson:"sender_id"`
	ReceiverID string             `bson:"receiver_id"`
	Currency   string             `bson:"currency"`
	Amount     float64            `bson:"amount"`
	State      string             `bson:"state"`
}

func NewTransfersMysqlRepository(cfg config.Mysql) *TransfersMysqlRepo {
	return &TransfersMysqlRepo{collection: "Values"}
}

func (r *TransfersMysqlRepo) Create(ctx context.Context, transfer models.Transfer) (string, error) {
	return "999", nil
}

func (r *TransfersMysqlRepo) GetByID(ctx context.Context, id string) (models.Transfer, error) {
	return models.Transfer{
		ID:         id,
		SenderID:   "SenderID",
		ReceiverID: "ReceiverID",
		Currency:   enums.CurrencyARS,
		Amount:     11.11,
		State:      "State",
	}, nil
}

func (r *TransfersMysqlRepo) Update(ctx context.Context, transfer models.Transfer) error {
	fmt.Println("Updated id")
	return nil
}

func (r *TransfersMysqlRepo) Delete(ctx context.Context, id string) error {
	fmt.Println("Deleted id")
	return nil
}
