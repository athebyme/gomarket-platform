package cache

import (
	"context"
	"fmt"
	"github.com/athebyme/gomarket-platform/pkg/errors"
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/go-redis/redis/v8"
	"time"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(ctx context.Context, host string, port int, password string, db int) (interfaces.CachePort, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Password:     password,
		DB:           db,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

func (r *RedisCache) buildKey(key, tenantID string) string {
	if tenantID != "" {
		return fmt.Sprintf("tenant:%s:%s", tenantID, key)
	}
	return key
}

func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, errors.ErrCacheMiss
		}
		return nil, err
	}
	return val, nil
}

func (r *RedisCache) GetWithTenant(ctx context.Context, key string, tenantID string) ([]byte, error) {
	return r.Get(ctx, r.buildKey(key, tenantID))
}

func (r *RedisCache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *RedisCache) SetWithTenant(ctx context.Context, key string, value []byte, tenantID string, expiration time.Duration) error {
	return r.Set(ctx, r.buildKey(key, tenantID), value, expiration)
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisCache) DeleteWithTenant(ctx context.Context, key string, tenantID string) error {
	return r.Delete(ctx, r.buildKey(key, tenantID))
}

func (r *RedisCache) DeleteByPattern(ctx context.Context, pattern string) error {
	iter := r.client.Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 100 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
			keys = keys[:0]
		}
	}

	if len(keys) > 0 {
		return r.client.Del(ctx, keys...).Err()
	}

	return iter.Err()
}

func (r *RedisCache) DeleteByPatternWithTenant(ctx context.Context, pattern string, tenantID string) error {
	tenantPattern := r.buildKey(pattern, tenantID)
	iter := r.client.Scan(ctx, 0, tenantPattern, 100).Iterator()
	var keys []string

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 100 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("ошибка при удалении ключей кэша: %w", err)
			}
			keys = keys[:0]
		}
	}

	if len(keys) > 0 {
		if err := r.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("ошибка при удалении оставшихся ключей кэша: %w", err)
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("ошибка при сканировании ключей по шаблону: %w", err)
	}

	return nil
}

func (r *RedisCache) Close() error {
	return r.client.Close()
}
