package repository

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRepo struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisRepo(addr, password string, db int, ttl time.Duration) *RedisRepo {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	return &RedisRepo{
		client: rdb,
		ttl:    ttl,
	}
}

func (r *RedisRepo) SetShortLink(ctx context.Context, shortCode, originalURL string, duration time.Duration) error {
	key := "short:" + shortCode
	if duration > 0 {
		return r.client.Set(ctx, key, originalURL, duration).Err()
	}
	return r.client.Set(ctx, key, originalURL, r.ttl).Err()
}

func (r *RedisRepo) GetOriginalURL(ctx context.Context, shortCode string) (string, error) {
	key := "short:" + shortCode
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // 未命中
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (r *RedisRepo) IncrementClick(ctx context.Context, shortCode string) (int64, error) {
	key := "click:" + shortCode
	return r.client.Incr(ctx, key).Result()
}

func (r *RedisRepo) GetClickCount(ctx context.Context, shortCode string) (int64, error) {
	key := "click:" + shortCode
	val, err := r.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return val, nil
}
