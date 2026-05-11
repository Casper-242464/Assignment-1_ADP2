package usecase

import (
	"context"
	"errors"

	"order-service/internal/domain"

	"github.com/google/uuid"
)

type OrderUsecase struct {
	repo      domain.OrderRepository
	cache     domain.OrderCache
	payment   domain.PaymentGateway
	statusHub *OrderStatusHub
}

func NewOrderUsecase(repo domain.OrderRepository, cache domain.OrderCache, payment domain.PaymentGateway, statusHub *OrderStatusHub) *OrderUsecase {
	return &OrderUsecase{repo: repo, cache: cache, payment: payment, statusHub: statusHub}
}

func (u *OrderUsecase) CreateOrder(ctx context.Context, customerID, customerEmail, itemName string, amount int64) (*domain.Order, error) {
	if amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	order := &domain.Order{
		ID:            uuid.NewString(),
		CustomerID:    customerID,
		CustomerEmail: customerEmail,
		ItemName:      itemName,
		Amount:        amount,
		Status:        domain.OrderStatusPending,
	}
	if err := u.repo.Create(ctx, order); err != nil {
		return nil, err
	}

	result, err := u.payment.Charge(ctx, order.ID, order.Amount, order.CustomerEmail)
	if err != nil {
		_ = u.repo.UpdateStatus(ctx, order.ID, domain.OrderStatusFailed)
		u.invalidateOrder(ctx, order.ID)
		order.Status = domain.OrderStatusFailed
		if u.statusHub != nil {
			u.statusHub.Publish(order.ID, order.Status)
		}
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
	u.invalidateOrder(ctx, order.ID)
	if u.statusHub != nil {
		u.statusHub.Publish(order.ID, order.Status)
	}

	return order, nil
}

func (u *OrderUsecase) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	if u.cache != nil {
		order, err := u.cache.Get(ctx, id)
		if err == nil {
			return order, nil
		}
	}

	order, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if u.cache != nil {
		_ = u.cache.Set(ctx, order)
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
	u.invalidateOrder(ctx, order.ID)
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

func (u *OrderUsecase) invalidateOrder(ctx context.Context, id string) {
	if u.cache != nil {
		_ = u.cache.Delete(ctx, id)
	}
}
