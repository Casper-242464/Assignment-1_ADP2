package usecase

import (
	"context"
	"errors"

	"order-service/internal/domain"

	"github.com/google/uuid"
)

type OrderUsecase struct {
	repo    domain.OrderRepository
	payment domain.PaymentGateway
}

func NewOrderUsecase(repo domain.OrderRepository, payment domain.PaymentGateway) *OrderUsecase {
	return &OrderUsecase{repo: repo, payment: payment}
}

func (u *OrderUsecase) CreateOrder(ctx context.Context, customerID, itemName string, amount int64) (*domain.Order, error) {
	if amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	order := &domain.Order{
		ID:         uuid.NewString(),
		CustomerID: customerID,
		ItemName:   itemName,
		Amount:     amount,
		Status:     domain.OrderStatusPending,
	}
	if err := u.repo.Create(ctx, order); err != nil {
		return nil, err
	}

	result, err := u.payment.Charge(ctx, order.ID, order.Amount)
	if err != nil {
		_ = u.repo.UpdateStatus(ctx, order.ID, domain.OrderStatusFailed)
		order.Status = domain.OrderStatusFailed
		if errors.Is(err, domain.ErrPaymentServiceUnavailable) {
			return order, domain.ErrPaymentServiceUnavailable
		}
		return order, err
	}

	if result.Status == "Authorized" {
		order.Status = domain.OrderStatusPaid
	} else {
		order.Status = domain.OrderStatusFailed
	}
	if err := u.repo.UpdateStatus(ctx, order.ID, order.Status); err != nil {
		return nil, err
	}

	return order, nil
}

func (u *OrderUsecase) CancelOrder(ctx context.Context, id string) (*domain.Order, error) {
	order, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if order.Status != domain.OrderStatusPending {
		return nil, domain.ErrInvalidOrderState
	}

	if err := u.repo.UpdateStatus(ctx, order.ID, domain.OrderStatusCancelled); err != nil {
		return nil, err
	}
	order.Status = domain.OrderStatusCancelled
	return order, nil
}

func (u *OrderUsecase) GetFilteredList(ctx context.Context, minAmount int64, maxAmount int64) ([]*domain.Order, error) {
	orders, err := u.repo.FilteredList(ctx, minAmount, maxAmount)
	if err != nil {
		return nil, err
	}
	return orders, nil
}
