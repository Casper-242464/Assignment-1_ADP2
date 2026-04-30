package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"payment-service/internal/domain"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQPublisher struct {
	conn  *amqp.Connection
	ch    *amqp.Channel
	queue string
	mu    sync.Mutex
}

func NewRabbitMQPublisher(url, queue string) (*RabbitMQPublisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare queue: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("enable publish confirms: %w", err)
	}

	return &RabbitMQPublisher{conn: conn, ch: ch, queue: queue}, nil
}

func (p *RabbitMQPublisher) PublishPaymentCompleted(ctx context.Context, event domain.PaymentCompletedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal payment event: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	confirms := p.ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	err = p.ch.PublishWithContext(ctx, "", p.queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    event.EventID,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
	if err != nil {
		return fmt.Errorf("publish payment event: %w", err)
	}

	select {
	case confirm := <-confirms:
		if !confirm.Ack {
			return fmt.Errorf("rabbitmq did not acknowledge published event %s", event.EventID)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timed out waiting for publish confirm")
	}
}

func (p *RabbitMQPublisher) Close() error {
	if p == nil {
		return nil
	}
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}
