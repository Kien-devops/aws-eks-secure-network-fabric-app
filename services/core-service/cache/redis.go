package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheClient struct {
	Client *redis.Client
}

func ConnectRedis(ctx context.Context, host, port, authToken string, useTLS bool) (*CacheClient, error) {
	addr := fmt.Sprintf("%s:%s", host, port)

	opts := &redis.Options{
		Addr:         addr,
		Password:     authToken,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
	}

	if useTLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed connecting to Redis (%s): %w", addr, err)
	}

	log.Printf("Connected successfully to ElastiCache Redis at %s (TLS: %v)", addr, useTLS)
	return &CacheClient{Client: rdb}, nil
}

func (c *CacheClient) SetOrderCache(ctx context.Context, orderID string, payload []byte, ttl time.Duration) error {
	key := fmt.Sprintf("order:%s", orderID)
	return c.Client.Set(ctx, key, payload, ttl).Err()
}

func (c *CacheClient) GetOrderCache(ctx context.Context, orderID string) (string, error) {
	key := fmt.Sprintf("order:%s", orderID)
	return c.Client.Get(ctx, key).Result()
}
