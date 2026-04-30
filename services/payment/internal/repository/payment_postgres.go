package repository

import (
	"context"
	"database/sql"
	"errors"

	"payment-service/internal/domain"
)

type PaymentPostgresRepository struct {
	db *sql.DB
}

func NewPaymentPostgresRepository(db *sql.DB) *PaymentPostgresRepository {
	return &PaymentPostgresRepository{db: db}
}

func (r *PaymentPostgresRepository) Create(ctx context.Context, payment *domain.Payment) error {
	const q = `
		INSERT INTO payments (id, order_id, amount, status, transaction_id)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.ExecContext(ctx, q, payment.ID, payment.OrderID, payment.Amount, payment.Status, nullableString(payment.TransactionID))
	return err
}

func (r *PaymentPostgresRepository) GetByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	const q = `
		SELECT id, order_id, amount, status, COALESCE(transaction_id, '')
		FROM payments
		WHERE order_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	var payment domain.Payment
	err := r.db.QueryRowContext(ctx, q, orderID).Scan(
		&payment.ID,
		&payment.OrderID,
		&payment.Amount,
		&payment.Status,
		&payment.TransactionID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, err
	}
	return &payment, nil
}

func (r *PaymentPostgresRepository) ListByStatus(ctx context.Context, status string) ([]*domain.Payment, error) {
	const q = `
		SELECT id, order_id, amount, status, COALESCE(transaction_id, '')
		FROM payments
		WHERE status = $1
		ORDER BY created_at DESC`

	var payments []*domain.Payment
	rows, err := r.db.QueryContext(ctx, q, status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, err
	}
	for rows.Next() {
		var payment domain.Payment
		if err := rows.Scan(
			&payment.ID,
			&payment.OrderID,
			&payment.Amount,
			&payment.Status,
			&payment.TransactionID,
		); err != nil {
			return nil, err
		}
		payments = append(payments, &payment)
	}
	return payments, nil
}

func nullableString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}
