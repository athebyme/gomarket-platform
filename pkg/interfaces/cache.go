package interfaces

import (
	"context"
	"errors"
	"time"
)

var (
	ErrCacheMiss = errors.New("cache miss")
)

// CachePort определяет интерфейс для работы с системой кэширования
// Реализация может использовать Redis, Memcached или любую другую систему кэширования
type CachePort interface {
	// Get получает значение из кэша по ключу
	// Возвращает nil, nil если значение не найдено
	Get(ctx context.Context, key string) ([]byte, error)

	// GetWithTenant получает значение из кэша по ключу с учетом ID арендатора
	// Помогает обеспечить изоляцию данных в многоарендной системе
	GetWithTenant(ctx context.Context, key string, tenantID string) ([]byte, error)

	// Set сохраняет значение в кэше с указанным сроком действия
	// Если expiration равно 0, срок действия не устанавливается
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error

	// SetWithTenant сохраняет значение в кэше с учетом ID арендатора
	SetWithTenant(ctx context.Context, key string, value []byte, tenantID string, expiration time.Duration) error

	// Delete удаляет значение из кэша по ключу
	Delete(ctx context.Context, key string) error

	// DeleteWithTenant удаляет значение из кэша по ключу с учетом ID арендатора
	DeleteWithTenant(ctx context.Context, key string, tenantID string) error

	// DeleteByPattern удаляет все значения, соответствующие шаблону
	// Например, "product:*" удалит все ключи, начинающиеся с "product:"
	DeleteByPattern(ctx context.Context, pattern string) error
	DeleteByPatternWithTenant(ctx context.Context, pattern, tenantID string) error

	// Close закрывает соединение с системой кэширования
	Close() error
}
