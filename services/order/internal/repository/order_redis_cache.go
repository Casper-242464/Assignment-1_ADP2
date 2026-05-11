package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"order-service/internal/domain"

	"github.com/redis/go-redis/v9"
)

type OrderRedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewOrderRedisCache(client *redis.Client, ttl time.Duration) *OrderRedisCache {
	return &OrderRedisCache{client: client, ttl: ttl}
}

func (c *OrderRedisCache) Get(ctx context.Context, id string) (*domain.Order, error) {
	body, err := c.client.Get(ctx, key(id)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}

	var order domain.Order
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (c *OrderRedisCache) Set(ctx context.Context, order *domain.Order) error {
	body, err := json.Marshal(order)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key(order.ID), body, c.ttl).Err()
}

func (c *OrderRedisCache) Delete(ctx context.Context, id string) error {
	return c.client.Del(ctx, key(id)).Err()
}

func key(id string) string {
	return fmt.Sprintf("orders:%s", id)
}
