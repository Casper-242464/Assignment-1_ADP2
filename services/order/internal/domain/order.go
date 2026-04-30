package domain

import "time"

const (
	OrderStatusPending   = "Pending"
	OrderStatusPaid      = "Paid"
	OrderStatusFailed    = "Failed"
	OrderStatusCancelled = "Cancelled"
)

type Order struct {
	ID            string
	CustomerID    string
	CustomerEmail string
	ItemName      string
	Amount        int64  // Amount in cents (e.g., 1000 = $10.00)
	Status        string // "Pending", "Paid", "Failed", "Cancelled"
	CreatedAt     time.Time
}

type PaymentResult struct {
	Status        string `json:"status"`
	TransactionID string `json:"transaction_id"`
}
