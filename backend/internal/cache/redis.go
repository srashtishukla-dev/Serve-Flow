package cache

import (
	"context"
	"errors"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/serveflow/serveflow/backend/internal/config"
)

var (
	ErrUnavailable = errors.New("redis cache is unavailable")
	ErrInvalidTTL  = errors.New("cache TTL must be positive")
)

type Client struct {
	client *redis.Client
	logger *log.Logger
}

func NewClient(settings config.Redis, logger *log.Logger) *Client {
	if logger == nil {
		logger = log.Default()
	}
	return &Client{
		client: redis.NewClient(&redis.Options{
			Addr:         net.JoinHostPort(settings.Host, strconv.Itoa(int(settings.Port))),
			DialTimeout:  250 * time.Millisecond,
			ReadTimeout:  250 * time.Millisecond,
			WriteTimeout: 250 * time.Millisecond,
			PoolTimeout:  250 * time.Millisecond,
			MaxRetries:   -1,
		}),
		logger: logger,
	}
}

func (client *Client) Connect(ctx context.Context) error {
	return client.Ping(ctx)
}

func (client *Client) Ping(ctx context.Context) error {
	if client == nil || client.client == nil {
		return ErrUnavailable
	}
	return client.client.Ping(ctx).Err()
}

func (client *Client) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if client == nil || client.client == nil {
		return nil, false, ErrUnavailable
	}
	value, err := client.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		client.logFailure("get", err)
		return nil, false, err
	}
	return value, true, nil
}

func (client *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if client == nil || client.client == nil {
		return ErrUnavailable
	}
	if ttl <= 0 {
		return ErrInvalidTTL
	}
	if err := client.client.Set(ctx, key, value, ttl).Err(); err != nil {
		client.logFailure("set", err)
		return err
	}
	return nil
}

func (client *Client) Delete(ctx context.Context, key string) error {
	if client == nil || client.client == nil {
		return ErrUnavailable
	}
	if err := client.client.Del(ctx, key).Err(); err != nil {
		client.logFailure("delete", err)
		return err
	}
	return nil
}

func (client *Client) Close() error {
	if client == nil || client.client == nil {
		return nil
	}
	return client.client.Close()
}

func (client *Client) logFailure(operation string, err error) {
	if client.logger != nil {
		client.logger.Printf("Redis cache %s failed: %v", operation, err)
	}
}
