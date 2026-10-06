// Package cache is the Redis cache-aside layer for redirect lookups.
//
// Key design:
//   - short:{code} -> original URL (positive entry, TTL 1h by default)
//   - short:{code} -> ""            (negative entry, short TTL, anti-penetration)
//
// Redis is treated as an optional accelerator: every error is returned to the
// caller, which logs and degrades to PostgreSQL instead of failing requests.
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// State describes the result of a cache lookup.
type State int

const (
	StateMiss     State = iota // key does not exist
	StateHit                   // positive entry, URL is valid
	StateNegative              // known-nonexistent code (cached empty marker)
)

// RedisCache wraps a go-redis client.
type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
	negTTL time.Duration
}

// New constructs the cache client without pinging (call Ping separately so a
// Redis outage degrades instead of crashing the process at boot).
func New(addr, password string, db int, ttl, negTTL time.Duration) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     20,
		MinIdleConns: 3,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
	})
	return &RedisCache{client: client, ttl: ttl, negTTL: negTTL}
}

// Ping verifies connectivity.
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close releases the connection pool.
func (c *RedisCache) Close() error { return c.client.Close() }

func (c *RedisCache) key(code string) string { return "short:" + code }

// SetLink writes a positive cache entry.
func (c *RedisCache) SetLink(ctx context.Context, code, originalURL string) error {
	return c.client.Set(ctx, c.key(code), originalURL, c.ttl).Err()
}

// SetNegative caches a known-nonexistent code with a short TTL.
func (c *RedisCache) SetNegative(ctx context.Context, code string) error {
	return c.client.Set(ctx, c.key(code), "", c.negTTL).Err()
}

// GetLink returns the cached state of a code. Redis errors surface to the
// caller as a miss-equivalent (state == StateMiss, err != nil).
func (c *RedisCache) GetLink(ctx context.Context, code string) (string, State, error) {
	val, err := c.client.Get(ctx, c.key(code)).Result()
	if errors.Is(err, redis.Nil) {
		return "", StateMiss, nil
	}
	if err != nil {
		return "", StateMiss, err
	}
	if val == "" {
		return "", StateNegative, nil
	}
	return val, StateHit, nil
}

// PoolStats exposes pool counters for the Prometheus gauge collector.
func (c *RedisCache) PoolStats() *redis.PoolStats {
	return c.client.PoolStats()
}
