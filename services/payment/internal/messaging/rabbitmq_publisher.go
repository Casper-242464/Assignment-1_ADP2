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

type internalChannel struct {
	ch       *amqp.Channel
	confirms <-chan amqp.Confirmation
}

type RabbitMQPublisher struct {
	conn      *amqp.Connection
	queue     string
	pool      chan *internalChannel
	closeOnce sync.Once
}

func NewRabbitMQPublisher(url, queue string, poolSize int) (*RabbitMQPublisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	publisher := &RabbitMQPublisher{
		conn:  conn,
		queue: queue,
		pool:  make(chan *internalChannel, poolSize),
	}

	for i := 0; i < poolSize; i++ {
		ich, err := publisher.createInternalChannel()
		if err != nil {
			_ = publisher.Close()
			return nil, err
		}
		publisher.pool <- ich
	}

	return publisher, nil
}

func (p *RabbitMQPublisher) createInternalChannel() (*internalChannel, error) {
	ch, err := p.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	if _, err := ch.QueueDeclare(p.queue, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("declare queue: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("enable publish confirms: %w", err)
	}

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	return &internalChannel{ch: ch, confirms: confirms}, nil
}

func (p *RabbitMQPublisher) PublishPaymentCompleted(ctx context.Context, event domain.PaymentCompletedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal payment event: %w", err)
	}

	var ich *internalChannel
	select {
	case ich = <-p.pool:
	case <-ctx.Done():
		return ctx.Err()
	}

	defer func() { p.pool <- ich }()

	err = ich.ch.PublishWithContext(ctx, "", p.queue, false, false, amqp.Publishing{
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
	case confirm := <-ich.confirms:
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
	p.closeOnce.Do(func() {
		if p.pool != nil {
			close(p.pool)
			for ich := range p.pool {
				_ = ich.ch.Close()
			}
		}
		if p.conn != nil {
			_ = p.conn.Close()
		}
	})
	return nil
}
