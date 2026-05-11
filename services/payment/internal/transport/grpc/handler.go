package grpc

import (
	"context"
	"errors"

	"payment-service/internal/domain"
	"payment-service/internal/usecase"

	paymentpb "github.com/Casper-242464/ConvertedProtosRepo/proto/payment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Handler struct {
	paymentpb.UnimplementedPaymentServiceServer
	uc *usecase.PaymentUsecase
}

func NewHandler(uc *usecase.PaymentUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) ProcessPayment(ctx context.Context, req *paymentpb.PaymentRequest) (*paymentpb.PaymentResponse, error) {
	payment, err := h.uc.CreatePayment(ctx, req.GetOrderId(), req.GetAmount(), customerEmailFromMetadata(ctx))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidAmount) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to process payment")
	}

	return &paymentpb.PaymentResponse{
		TransactionId: payment.TransactionID,
		Status:        payment.Status,
	}, nil
}

func customerEmailFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get("customer-email")
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (h *Handler) ListPayments(ctx context.Context, req *paymentpb.ListPaymentsRequest) (*paymentpb.ListPaymentResponse, error) {
	payments, err := h.uc.ListByStatus(ctx, req.GetStatus())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list payments")
	}

	var paymentProtos []*paymentpb.PaymentResponse
	for _, payment := range payments {
		paymentProtos = append(paymentProtos, &paymentpb.PaymentResponse{
			TransactionId: payment.TransactionID,
			Status:        payment.Status,
		})
	}

	return &paymentpb.ListPaymentResponse{
		Payments: paymentProtos,
	}, nil
}
