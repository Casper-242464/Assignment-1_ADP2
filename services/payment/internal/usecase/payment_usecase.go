package usecase

import (
	"context"

	"payment-service/internal/domain"

	"github.com/google/uuid"
)

type PaymentUsecase struct {
	repo domain.PaymentRepository
}

func NewPaymentUsecase(repo domain.PaymentRepository) *PaymentUsecase {
	return &PaymentUsecase{repo: repo}
}

func (u *PaymentUsecase) CreatePayment(ctx context.Context, orderID string, amount int64) (*domain.Payment, error) {
	if amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	payment := &domain.Payment{
		ID:      uuid.NewString(),
		OrderID: orderID,
		Amount:  amount,
	}

	if amount > 100000 {
		payment.Status = domain.PaymentStatusDeclined
	} else {
		payment.Status = domain.PaymentStatusAuthorized
		payment.TransactionID = uuid.NewString()
	}

	if err := u.repo.Create(ctx, payment); err != nil {
		return nil, err
	}

	return payment, nil
}

func (u *PaymentUsecase) GetByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	return u.repo.GetByOrderID(ctx, orderID)
}

func (u *PaymentUsecase) ListByStatus(ctx context.Context, status string) ([]*domain.Payment, error) {
	return u.repo.ListByStatus(ctx, status)
}
