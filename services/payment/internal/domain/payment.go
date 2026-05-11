package domain

const (
	PaymentStatusAuthorized = "Authorized"
	PaymentStatusDeclined   = "Declined"
)

type Payment struct {
	ID            string
	OrderID       string
	CustomerEmail string
	TransactionID string
	Amount        int64  // Amount in cents
	Status        string // "Authorized", "Declined"
}

type PaymentCompletedEvent struct {
	EventID       string `json:"event_id"`
	PaymentID     string `json:"payment_id"`
	OrderID       string `json:"order_id"`
	Amount        int64  `json:"amount"`
	CustomerEmail string `json:"customer_email"`
	Status        string `json:"status"`
}
