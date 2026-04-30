package grpc

import (
	"context"
	"errors"

	"order-service/internal/domain"

	paymentpb "github.com/Casper-242464/ConvertedProtosRepo/proto/payment"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type PaymentClient struct {
	client paymentpb.PaymentServiceClient
}

func NewPaymentClient(conn *grpcLib.ClientConn) *PaymentClient {
	return &PaymentClient{client: paymentpb.NewPaymentServiceClient(conn)}
}

func (p *PaymentClient) Charge(ctx context.Context, orderID string, amount int64, customerEmail string) (*domain.PaymentResult, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "customer-email", customerEmail)
	resp, err := p.client.ProcessPayment(ctx, &paymentpb.PaymentRequest{OrderId: orderID, Amount: amount})
	if err != nil {
		return nil, domain.ErrPaymentServiceUnavailable
	}
	if resp == nil || resp.GetStatus() == "" {
		return nil, errors.New("invalid payment service response")
	}

	return &domain.PaymentResult{
		Status:        resp.GetStatus(),
		TransactionID: resp.GetTransactionId(),
	}, nil
}
