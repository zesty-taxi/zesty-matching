package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zesty-taxi/zesty-matching/internal/domain"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(addr, password string, db int) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

// -----------------------------------------------
// Driver Lock
// Ставим перед отправкой оффера водителю.
// Если lock уже есть — водитель занят другим матчингом.
// -----------------------------------------------

func (c *Client) AcquireDriverLock(ctx context.Context, driverID, rideID string) (bool, error) {
	key := driverLockKey(driverID)
	// SETNX — атомарно: ставит только если ключа нет
	ok, err := c.rdb.SetNX(ctx, key, rideID, domain.DriverLockTTL).Result()
	if err != nil {
		return false, fmt.Errorf("acquire driver lock: %w", err)
	}
	return ok, nil
}

func (c *Client) ReleaseDriverLock(ctx context.Context, driverID string) error {
	key := driverLockKey(driverID)
	if err := c.rdb.Del(ctx, key).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("release driver lock: %w", err)
	}
	return nil
}

// -----------------------------------------------
// Match Queue
// Список кандидатов для поездки.
// Когда водитель отказал — берём следующего из очереди.
// -----------------------------------------------

func (c *Client) SaveMatchQueue(ctx context.Context, queue domain.MatchQueue) error {
	data, err := json.Marshal(queue)
	if err != nil {
		return fmt.Errorf("marshal match queue: %w", err)
	}

	key := matchQueueKey(queue.RideID)
	if err := c.rdb.Set(ctx, key, data, domain.MatchingTimeout).Err(); err != nil {
		return fmt.Errorf("save match queue: %w", err)
	}
	return nil
}

func (c *Client) GetMatchQueue(ctx context.Context, rideID string) (*domain.MatchQueue, error) {
	key := matchQueueKey(rideID)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // очередь не найдена — поездка уже завершена/отменена
		}
		return nil, fmt.Errorf("get match queue: %w", err)
	}

	var queue domain.MatchQueue
	if err := json.Unmarshal(data, &queue); err != nil {
		return nil, fmt.Errorf("unmarshal match queue: %w", err)
	}
	return &queue, nil
}

func (c *Client) DeleteMatchQueue(ctx context.Context, rideID string) error {
	key := matchQueueKey(rideID)
	if err := c.rdb.Del(ctx, key).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("delete match queue: %w", err)
	}
	return nil
}

// -----------------------------------------------
// Active Offer
// Текущий активный оффер для поездки.
// Нужен чтобы проверить что ответ водителя
// соответствует актуальному offerID.
// -----------------------------------------------

func (c *Client) SaveActiveOffer(ctx context.Context, offer domain.ActiveOffer) error {
	data, err := json.Marshal(offer)
	if err != nil {
		return fmt.Errorf("marshal active offer: %w", err)
	}

	key := activeOfferKey(offer.RideID)
	if err := c.rdb.Set(ctx, key, data, domain.OfferTTL).Err(); err != nil {
		return fmt.Errorf("save active offer: %w", err)
	}
	return nil
}

func (c *Client) GetActiveOffer(ctx context.Context, rideID string) (*domain.ActiveOffer, error) {
	key := activeOfferKey(rideID)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // оффер истёк или не существует
		}
		return nil, fmt.Errorf("get active offer: %w", err)
	}

	var offer domain.ActiveOffer
	if err := json.Unmarshal(data, &offer); err != nil {
		return nil, fmt.Errorf("unmarshal active offer: %w", err)
	}
	return &offer, nil
}

func (c *Client) DeleteActiveOffer(ctx context.Context, rideID string) error {
	key := activeOfferKey(rideID)
	if err := c.rdb.Del(ctx, key).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("delete active offer: %w", err)
	}
	return nil
}

// -----------------------------------------------
// Keys
// -----------------------------------------------

func driverLockKey(driverID string) string {
	return fmt.Sprintf("lock:driver:%s", driverID)
}

func matchQueueKey(rideID string) string {
	return fmt.Sprintf("match:queue:%s", rideID)
}

func activeOfferKey(rideID string) string {
	return fmt.Sprintf("offer:active:%s", rideID)
}
