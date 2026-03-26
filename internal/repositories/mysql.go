package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"transfers-api/internal/config"
	"transfers-api/internal/enums"
	"transfers-api/internal/known_errors"
	"transfers-api/internal/logging"
	"transfers-api/internal/models"
)

type TransfersMySQLRepo struct {
	db *sql.DB
}

type transferMySQLDAO struct {
	ID         int64
	SenderID   string
	ReceiverID string
	Currency   string
	Amount     float64
	State      string
}

func NewTransfersMySQLRepository(cfg config.MySQL) *TransfersMySQLRepo {

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.Username,
		cfg.Password,
		cfg.Hostname,
		cfg.Port,
		cfg.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		logging.Logger.Fatalf("error connecting to MySQL: %v", err)
	}

	if err := db.Ping(); err != nil {
		logging.Logger.Fatalf("error pinging MySQL: %v", err)
	}

	return &TransfersMySQLRepo{
		db: db,
	}
}

func (r *TransfersMySQLRepo) Create(ctx context.Context, transfer models.Transfer) (string, error) {

	dao := transferMySQLDAO{
		SenderID:   transfer.SenderID,
		ReceiverID: transfer.ReceiverID,
		Currency:   transfer.Currency.String(),
		Amount:     transfer.Amount,
		State:      transfer.State,
	}

	query := `
	INSERT INTO transfers
	(sender_id, receiver_id, currency, amount, state)
	VALUES (?, ?, ?, ?, ?)`

	res, err := r.db.ExecContext(
		ctx,
		query,
		dao.SenderID,
		dao.ReceiverID,
		dao.Currency,
		dao.Amount,
		dao.State,
	)

	if err != nil {
		return "", fmt.Errorf("error inserting transfer in MySQL: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return "", fmt.Errorf("error retrieving inserted id: %w", err)
	}

	return fmt.Sprintf("%d", id), nil
}

func (r *TransfersMySQLRepo) GetByID(ctx context.Context, id string) (models.Transfer, error) {

	var dao transferMySQLDAO

	query := `
	SELECT id, sender_id, receiver_id, currency, amount, state
	FROM transfers
	WHERE id = ?`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&dao.ID,
		&dao.SenderID,
		&dao.ReceiverID,
		&dao.Currency,
		&dao.Amount,
		&dao.State,
	)

	if err != nil {

		if errors.Is(err, sql.ErrNoRows) {
			return models.Transfer{}, fmt.Errorf("transfer not found: %w", known_errors.ErrNotFound)
		}

		return models.Transfer{}, fmt.Errorf("error getting transfer: %w", err)
	}

	return models.Transfer{
		ID:         fmt.Sprintf("%d", dao.ID),
		SenderID:   dao.SenderID,
		ReceiverID: dao.ReceiverID,
		Currency:   enums.ParseCurrency(dao.Currency),
		Amount:     dao.Amount,
		State:      dao.State, // TODO: replace with enums.ParseState
	}, nil
}

func (r *TransfersMySQLRepo) Update(ctx context.Context, transfer models.Transfer) error {

	setClauses := []string{}
	args := []interface{}{}

	if transfer.SenderID != "" {
		setClauses = append(setClauses, "sender_id = ?")
		args = append(args, transfer.SenderID)
	}

	if transfer.ReceiverID != "" {
		setClauses = append(setClauses, "receiver_id = ?")
		args = append(args, transfer.ReceiverID)
	}

	if transfer.Currency != enums.CurrencyUnknown {
		setClauses = append(setClauses, "currency = ?")
		args = append(args, transfer.Currency.String())
	}

	if transfer.Amount != 0 {
		setClauses = append(setClauses, "amount = ?")
		args = append(args, transfer.Amount)
	}

	if transfer.State != "" {
		setClauses = append(setClauses, "state = ?")
		args = append(args, transfer.State)
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no valid fields to update: %w", known_errors.ErrBadRequest)
	}

	query := fmt.Sprintf(
		"UPDATE transfers SET %s WHERE id = ?",
		joinClauses(setClauses),
	)

	args = append(args, transfer.ID)

	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("error updating transfer: %w", err)
	}

	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return fmt.Errorf("transfer not found: %w", known_errors.ErrNotFound)
	}

	return nil
}

func (r *TransfersMySQLRepo) Delete(ctx context.Context, id string) error {

	query := `DELETE FROM transfers WHERE id = ?`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting transfer: %w", err)
	}

	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return fmt.Errorf("transfer not found: %w", known_errors.ErrNotFound)
	}

	return nil
}

func joinClauses(clauses []string) string {
	out := ""
	for i, c := range clauses {
		if i > 0 {
			out += ", "
		}
		out += c
	}
	return out
}

func (r *TransfersMySQLRepo) GetByUserID(ctx context.Context, id string) ([]models.Transfer, error) {
	var transferResult []models.Transfer
	return  transferResult, nil
}

