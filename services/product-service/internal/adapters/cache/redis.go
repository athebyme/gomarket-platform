package cache

import (
	"context"
	"fmt"
	"github.com/athebyme/gomarket-platform/pkg/errors"
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/go-redis/redis/v8"
	"sync"
	"time"
)

// RedisCache реализация CachePort с использованием Redis
type RedisCache struct {
	client *redis.Client
	mu     sync.RWMutex // для потокобезопасного доступа
}

// NewRedisCache создает новый экземпляр RedisCache
func NewRedisCache(ctx context.Context, host string, port int, password string, db int, poolSize int, minIdleConns int) (interfaces.CachePort, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Password:     password,
		DB:           db,
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
		// Дополнительные настройки для оптимизации производительности
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolTimeout:     4 * time.Second,
		IdleTimeout:     5 * time.Minute,
	})

	// Проверяем соединение
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("не удалось подключиться к Redis: %w", err)
	}

	return &RedisCache{
		client: client,
		mu:     sync.RWMutex{},
	}, nil
}

// buildKey строит ключ с учетом арендатора
func (r *RedisCache) buildKey(key, tenantID string) string {
	if tenantID != "" {
		return fmt.Sprintf("tenant:%s:%s", tenantID, key)
	}
	return key
}

// Get получает значение из кэша по ключу
func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	val, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, errors.ErrCacheMiss
		}
		return nil, err
	}
	return val, nil
}

// GetWithTenant получает значение из кэша по ключу с учетом ID арендатора
func (r *RedisCache) GetWithTenant(ctx context.Context, key string, tenantID string) ([]byte, error) {
	return r.Get(ctx, r.buildKey(key, tenantID))
}

// Set сохраняет значение в кэше с указанным сроком действия
func (r *RedisCache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.client.Set(ctx, key, value, expiration).Err()
}

// SetWithTenant сохраняет значение в кэше с учетом ID арендатора
func (r *RedisCache) SetWithTenant(ctx context.Context, key string, value []byte, tenantID string, expiration time.Duration) error {
	return r.Set(ctx, r.buildKey(key, tenantID), value, expiration)
}

// Delete удаляет значение из кэша по ключу
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.client.Del(ctx, key).Err()
}

// DeleteWithTenant удаляет значение из кэша по ключу с учетом ID арендатора
func (r *RedisCache) DeleteWithTenant(ctx context.Context, key string, tenantID string) error {
	return r.Delete(ctx, r.buildKey(key, tenantID))
}

// DeleteByPattern удаляет все значения, соответствующие шаблону
func (r *RedisCache) DeleteByPattern(ctx context.Context, pattern string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	iter := r.client.Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		// Удаляем пакетами по 100 ключей, чтобы не нагружать Redis
		if len(keys) >= 100 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
			keys = keys[:0]
		}
	}

	// Удаляем оставшиеся ключи
	if len(keys) > 0 {
		return r.client.Del(ctx, keys...).Err()
	}

	return iter.Err()
}

// DeleteByPatternWithTenant удаляет все значения, соответствующие шаблону с учетом ID арендатора
func (r *RedisCache) DeleteByPatternWithTenant(ctx context.Context, pattern string, tenantID string) error {
	return r.DeleteByPattern(ctx, r.buildKey(pattern, tenantID))
}

// GetMulti получает несколько значений за один запрос
func (r *RedisCache) GetMulti(ctx context.Context, keys []string) (map[string][]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]byte, len(keys))
	pipe := r.client.Pipeline()
	cmds := make(map[string]*redis.StringCmd, len(keys))

	// Добавляем все команды в пайплайн
	for _, key := range keys {
		cmds[key] = pipe.Get(ctx, key)
	}

	// Выполняем пайплайн
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	// Обрабатываем результаты
	for key, cmd := range cmds {
		val, err := cmd.Bytes()
		if err == nil {
			result[key] = val
		} else if err != redis.Nil {
			return nil, err
		}
	}

	return result, nil
}

// GetMultiWithTenant получает несколько значений за один запрос с учетом ID арендатора
func (r *RedisCache) GetMultiWithTenant(ctx context.Context, keys []string, tenantID string) (map[string][]byte, error) {
	tenantKeys := make([]string, len(keys))
	for i, key := range keys {
		tenantKeys[i] = r.buildKey(key, tenantID)
	}

	// Получаем значения с учетом тенанта
	results, err := r.GetMulti(ctx, tenantKeys)
	if err != nil {
		return nil, err
	}

	// Преобразуем обратно к оригинальным ключам
	originalResults := make(map[string][]byte, len(results))
	prefix := fmt.Sprintf("tenant:%s:", tenantID)
	prefixLen := len(prefix)

	for k, v := range results {
		if len(k) > prefixLen && k[:prefixLen] == prefix {
			originalResults[k[prefixLen:]] = v
		} else {
			originalResults[k] = v
		}
	}

	return originalResults, nil
}

// SetMulti сохраняет несколько значений за один запрос
func (r *RedisCache) SetMulti(ctx context.Context, items map[string][]byte, expiration time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	pipe := r.client.Pipeline()
	for key, value := range items {
		pipe.Set(ctx, key, value, expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// SetMultiWithTenant сохраняет несколько значений за один запрос с учетом ID арендатора
func (r *RedisCache) SetMultiWithTenant(ctx context.Context, items map[string][]byte, tenantID string, expiration time.Duration) error {
	tenantItems := make(map[string][]byte, len(items))
	for key, value := range items {
		tenantItems[r.buildKey(key, tenantID)] = value
	}
	return r.SetMulti(ctx, tenantItems, expiration)
}

// Increment увеличивает числовое значение ключа на указанную величину
func (r *RedisCache) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.client.IncrBy(ctx, key, delta).Result()
}

// IncrementWithTenant увеличивает числовое значение ключа с учетом ID арендатора
func (r *RedisCache) IncrementWithTenant(ctx context.Context, key string, tenantID string, delta int64) (int64, error) {
	return r.Increment(ctx, r.buildKey(key, tenantID), delta)
}

// Lock пытается получить блокировку с указанным ключом
func (r *RedisCache) Lock(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Используем SET с NX флагом для atomic lock
	return r.client.SetNX(ctx, "lock:"+key, "1", expiration).Result()
}

// LockWithTenant пытается получить блокировку с учетом ID арендатора
func (r *RedisCache) LockWithTenant(ctx context.Context, key string, tenantID string, expiration time.Duration) (bool, error) {
	return r.Lock(ctx, r.buildKey(key, tenantID), expiration)
}

// Unlock освобождает блокировку
func (r *RedisCache) Unlock(ctx context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.client.Del(ctx, "lock:"+key).Err()
}

// UnlockWithTenant освобождает блокировку с учетом ID арендатора
func (r *RedisCache) UnlockWithTenant(ctx context.Context, key string, tenantID string) error {
	return r.Unlock(ctx, r.buildKey(key, tenantID))
}

// Close закрывает соединение с системой кэширования
func (r *RedisCache) Close() error {
	return r.client.Close()
}
