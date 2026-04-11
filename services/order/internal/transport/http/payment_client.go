package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"order-service/internal/domain"
)

type PaymentClient struct {
	baseURL string
	client  *http.Client
}

func NewPaymentClient(baseURL string, client *http.Client) *PaymentClient {
	return &PaymentClient{baseURL: baseURL, client: client}
}

type paymentRequest struct {
	OrderID string `json:"order_id"`
	Amount  int64  `json:"amount"`
}

type paymentResponse struct {
	Status        string `json:"status"`
	TransactionID string `json:"transaction_id"`
}

func (p *PaymentClient) Charge(ctx context.Context, orderID string, amount int64) (*domain.PaymentResult, error) {
	payload := paymentRequest{OrderID: orderID, Amount: amount}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/payments", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, domain.ErrPaymentServiceUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, domain.ErrPaymentServiceUnavailable
	}
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("payment service returned %d: %s", resp.StatusCode, string(raw))
	}

	var result paymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Status == "" {
		return nil, errors.New("invalid payment service response")
	}

	return &domain.PaymentResult{
		Status:        result.Status,
		TransactionID: result.TransactionID,
	}, nil
}
