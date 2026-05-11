package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

const (
	statusProcessing = "processing"
	statusSent       = "sent"
)

type PaymentCompletedEvent struct {
	EventID       string `json:"event_id"`
	PaymentID     string `json:"payment_id"`
	OrderID       string `json:"order_id"`
	Amount        int64  `json:"amount"`
	CustomerEmail string `json:"customer_email"`
	Status        string `json:"status"`
}

type NotificationProvider interface {
	SendPaymentNotification(ctx context.Context, event PaymentCompletedEvent) error
}

type SimulatedEmailProvider struct {
	latency     time.Duration
	failureRate float64
}

func NewSimulatedEmailProvider(latency time.Duration, failureRate float64) *SimulatedEmailProvider {
	return &SimulatedEmailProvider{latency: latency, failureRate: failureRate}
}

func (p *SimulatedEmailProvider) SendPaymentNotification(ctx context.Context, event PaymentCompletedEvent) error {
	select {
	case <-time.After(p.latency):
	case <-ctx.Done():
		return ctx.Err()
	}

	if rand.Float64() < p.failureRate {
		return errors.New("simulated provider timeout")
	}

	log.Printf("[Notification] Sent email to %s for Order #%s. Amount: %s. Status: %s",
		event.CustomerEmail,
		event.OrderID,
		formatCents(event.Amount),
		event.Status,
	)
	return nil
}

type RedisIdempotencyStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisIdempotencyStore(client *redis.Client, ttl time.Duration) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{client: client, ttl: ttl}
}

func (s *RedisIdempotencyStore) Begin(ctx context.Context, paymentID string) (bool, error) {
	key := idempotencyKey(paymentID)
	status, err := s.client.Get(ctx, key).Result()
	if err == nil && status == statusSent {
		return false, nil
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		return false, err
	}
	if err := s.client.Set(ctx, key, statusProcessing, s.ttl).Err(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *RedisIdempotencyStore) MarkSent(ctx context.Context, paymentID string) error {
	return s.client.Set(ctx, idempotencyKey(paymentID), statusSent, s.ttl).Err()
}

func main() {
	rand.Seed(time.Now().UnixNano())

	rabbitURL := envOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	queueName := envOrDefault("PAYMENT_COMPLETED_QUEUE", "payment.completed")
	redisAddress := envOrDefault("REDIS_ADDR", "localhost:6379")
	idempotencyTTL := envDurationOrDefault("NOTIFICATION_IDEMPOTENCY_TTL", 24*time.Hour)
	maxRetries := envIntOrDefault("NOTIFICATION_MAX_RETRIES", 3)
	initialBackoff := envDurationOrDefault("NOTIFICATION_INITIAL_BACKOFF", 2*time.Second)
	provider := buildProvider()

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddress})
	defer redisClient.Close()
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("ping redis: %v", err)
	}
	store := NewRedisIdempotencyStore(redisClient, idempotencyTTL)

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
			handleDelivery(ctx, delivery, store, provider, maxRetries, initialBackoff)
		}
	}
}

func buildProvider() NotificationProvider {
	mode := strings.ToUpper(envOrDefault("PROVIDER_MODE", "SIMULATED"))
	switch mode {
	case "SIMULATED":
		latency := envDurationOrDefault("SIMULATED_PROVIDER_LATENCY", 750*time.Millisecond)
		failureRate := envFloatOrDefault("SIMULATED_PROVIDER_FAILURE_RATE", 0.3)
		return NewSimulatedEmailProvider(latency, failureRate)
	default:
		log.Fatalf("unsupported PROVIDER_MODE %q", mode)
		return nil
	}
}

func handleDelivery(ctx context.Context, delivery amqp.Delivery, store *RedisIdempotencyStore, provider NotificationProvider, maxRetries int, initialBackoff time.Duration) {
	var event PaymentCompletedEvent
	if err := json.Unmarshal(delivery.Body, &event); err != nil {
		log.Printf("invalid payment event: %v", err)
		_ = delivery.Nack(false, false)
		return
	}

	paymentID := event.PaymentID
	if paymentID == "" {
		paymentID = event.EventID
	}
	if paymentID == "" {
		paymentID = delivery.MessageId
	}
	if paymentID == "" {
		log.Printf("payment event missing event_id")
		_ = delivery.Nack(false, false)
		return
	}

	shouldProcess, err := store.Begin(ctx, paymentID)
	if err != nil {
		log.Printf("check idempotency for %s: %v", paymentID, err)
		_ = delivery.Nack(false, true)
		return
	}
	if !shouldProcess {
		log.Printf("[Notification] Duplicate payment %s skipped", paymentID)
		_ = delivery.Ack(false)
		return
	}

	if err := sendWithRetry(ctx, provider, event, maxRetries, initialBackoff); err != nil {
		log.Printf("notification failed for payment %s: %v", paymentID, err)
		_ = delivery.Nack(false, true)
		return
	}

	if err := store.MarkSent(ctx, paymentID); err != nil {
		log.Printf("mark payment %s sent: %v", paymentID, err)
		_ = delivery.Nack(false, true)
		return
	}

	if err := delivery.Ack(false); err != nil {
		log.Printf("ack event %s: %v", paymentID, err)
	}
}

func sendWithRetry(ctx context.Context, provider NotificationProvider, event PaymentCompletedEvent, maxRetries int, initialBackoff time.Duration) error {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := provider.SendPaymentNotification(ctx, event); err != nil {
			lastErr = err
			if attempt == maxRetries {
				break
			}
			delay := initialBackoff * time.Duration(1<<(attempt-1))
			log.Printf("notification attempt %d/%d failed: %v; retrying in %s", attempt, maxRetries, err, delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}
		return nil
	}
	return lastErr
}

func idempotencyKey(paymentID string) string {
	return fmt.Sprintf("notifications:payment:%s", paymentID)
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

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Fatalf("%s must be a valid duration: %v", key, err)
	}
	return duration
}

func envIntOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		log.Fatalf("%s must be a positive integer", key)
	}
	return parsed
}

func envFloatOrDefault(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 || parsed > 1 {
		log.Fatalf("%s must be a decimal between 0 and 1", key)
	}
	return parsed
}
