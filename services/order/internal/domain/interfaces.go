package domain

import (
	"context"
	"errors"
)

var (
	ErrOrderNotFound             = errors.New("order not found")
	ErrInvalidAmount             = errors.New("invalid amount")
	ErrInvalidOrderState         = errors.New("invalid order state")
	ErrPaymentServiceUnavailable = errors.New("payment service unavailable")
)

type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, id string) (*Order, error)
	UpdateStatus(ctx context.Context, id string, status string) error
	FilteredList(ctx context.Context, minAmount int64, maxAmount int64) ([]*Order, error)
}

type PaymentGateway interface {
	Charge(ctx context.Context, orderID string, amount int64) (*PaymentResult, error)
}
