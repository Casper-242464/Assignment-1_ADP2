package repository

import (
	"context"
	"database/sql"
	"errors"

	"order-service/internal/domain"
)

type OrderPostgresRepository struct {
	db *sql.DB
}

func NewOrderPostgresRepository(db *sql.DB) *OrderPostgresRepository {
	return &OrderPostgresRepository{db: db}
}

func (r *OrderPostgresRepository) Create(ctx context.Context, order *domain.Order) error {
	const q = `
		INSERT INTO orders (id, customer_id, customer_email, item_name, amount, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at`

	return r.db.QueryRowContext(ctx, q, order.ID, order.CustomerID, order.CustomerEmail, order.ItemName, order.Amount, order.Status).Scan(&order.CreatedAt)
}

func (r *OrderPostgresRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	const q = `
		SELECT id, customer_id, customer_email, item_name, amount, status, created_at
		FROM orders
		WHERE id = $1`

	var order domain.Order
	err := r.db.QueryRowContext(ctx, q, id).Scan(&order.ID, &order.CustomerID, &order.CustomerEmail, &order.ItemName, &order.Amount, &order.Status, &order.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

func (r *OrderPostgresRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	const q = `
		UPDATE orders
		SET status = $1
		WHERE id = $2`

	res, err := r.db.ExecContext(ctx, q, status, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrOrderNotFound
	}
	return nil
}

func (r *OrderPostgresRepository) FilteredList(ctx context.Context, minAmount int64, maxAmount int64) ([]*domain.Order, error) {
	const q = `
		SELECT id, customer_id, customer_email, item_name, amount, status, created_at
		FROM orders
		WHERE amount BETWEEN $1 AND $2;`

	rows, err := r.db.QueryContext(ctx, q, minAmount, maxAmount)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		var order domain.Order
		if err := rows.Scan(&order.ID, &order.CustomerID, &order.CustomerEmail, &order.ItemName, &order.Amount, &order.Status, &order.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, &order)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, domain.ErrOrderNotFound
	}
	return orders, nil
}
