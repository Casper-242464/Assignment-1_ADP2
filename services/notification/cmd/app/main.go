package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	amqp "github.com/rabbitmq/amqp091-go"
)

type PaymentCompletedEvent struct {
	EventID       string `json:"event_id"`
	OrderID       string `json:"order_id"`
	Amount        int64  `json:"amount"`
	CustomerEmail string `json:"customer_email"`
	Status        string `json:"status"`
}

type IdempotencyStore struct {
	mu        sync.Mutex
	processed map[string]struct{}
}

func NewIdempotencyStore() *IdempotencyStore {
	return &IdempotencyStore{processed: make(map[string]struct{})}
}

func (s *IdempotencyStore) Seen(eventID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.processed[eventID]; ok {
		return true
	}
	s.processed[eventID] = struct{}{}
	return false
}

func main() {
	rabbitURL := envOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	queueName := envOrDefault("PAYMENT_COMPLETED_QUEUE", "payment.completed")

	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		log.Fatalf("dial rabbitmq: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("open rabbitmq channel: %v", err)
	}
	defer ch.Close()

	if _, err := ch.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		log.Fatalf("declare queue: %v", err)
	}

	if err := ch.Qos(1, 0, false); err != nil {
		log.Fatalf("set qos: %v", err)
	}

	deliveries, err := ch.Consume(queueName, "notification-service", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("consume queue: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := NewIdempotencyStore()
	log.Printf("notification service waiting for messages on %s", queueName)

	for {
		select {
		case <-ctx.Done():
			log.Println("shutting down notification service")
			return
		case delivery, ok := <-deliveries:
			if !ok {
				return
			}
			handleDelivery(delivery, store)
		}
	}
}

func handleDelivery(delivery amqp.Delivery, store *IdempotencyStore) {
	var event PaymentCompletedEvent
	if err := json.Unmarshal(delivery.Body, &event); err != nil {
		log.Printf("invalid payment event: %v", err)
		_ = delivery.Nack(false, false)
		return
	}

	eventID := event.EventID
	if eventID == "" {
		eventID = delivery.MessageId
	}
	if eventID == "" {
		log.Printf("payment event missing event_id")
		_ = delivery.Nack(false, false)
		return
	}

	if store.Seen(eventID) {
		log.Printf("[Notification] Duplicate event %s skipped", eventID)
		_ = delivery.Ack(false)
		return
	}

	log.Printf("[Notification] Sent email to %s for Order #%s. Amount: %s. Status: %s",
		event.CustomerEmail,
		event.OrderID,
		formatCents(event.Amount),
		event.Status,
	)

	if err := delivery.Ack(false); err != nil {
		log.Printf("ack event %s: %v", eventID, err)
	}
}

func formatCents(amount int64) string {
	return fmt.Sprintf("$%d.%02d", amount/100, amount%100)
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
