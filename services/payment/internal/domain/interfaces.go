package domain

import (
	"context"
	"errors"
)

var (
	ErrPaymentNotFound = errors.New("payment not found")
	ErrInvalidAmount   = errors.New("invalid amount")
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *Payment) error
	GetByOrderID(ctx context.Context, orderID string) (*Payment, error)
}
