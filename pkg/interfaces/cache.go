package interfaces

import (
	"context"
	"time"
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

	// DeleteByPatternWithTenant удаляет все значения, соответствующие шаблону с учетом ID арендатора
	DeleteByPatternWithTenant(ctx context.Context, pattern string, tenantID string) error

	// GetMulti получает несколько значений за один запрос (для оптимизации)
	// Возвращает map, где ключи - запрошенные ключи, а значения - данные из кэша
	// Если какой-то ключ не найден, он отсутствует в результирующей map
	GetMulti(ctx context.Context, keys []string) (map[string][]byte, error)

	// GetMultiWithTenant получает несколько значений за один запрос с учетом ID арендатора
	GetMultiWithTenant(ctx context.Context, keys []string, tenantID string) (map[string][]byte, error)

	// SetMulti сохраняет несколько значений за один запрос
	SetMulti(ctx context.Context, items map[string][]byte, expiration time.Duration) error

	// SetMultiWithTenant сохраняет несколько значений за один запрос с учетом ID арендатора
	SetMultiWithTenant(ctx context.Context, items map[string][]byte, tenantID string, expiration time.Duration) error

	// Increment увеличивает числовое значение ключа на указанную величину
	// Если ключ не существует, он будет создан со значением delta
	// Возвращает новое значение
	Increment(ctx context.Context, key string, delta int64) (int64, error)

	// IncrementWithTenant увеличивает числовое значение ключа с учетом ID арендатора
	IncrementWithTenant(ctx context.Context, key string, tenantID string, delta int64) (int64, error)

	// Lock пытается получить блокировку с указанным ключом
	// Возвращает true, если блокировка получена успешно
	// Может использоваться для распределенных блокировок
	Lock(ctx context.Context, key string, expiration time.Duration) (bool, error)

	// LockWithTenant пытается получить блокировку с учетом ID арендатора
	LockWithTenant(ctx context.Context, key string, tenantID string, expiration time.Duration) (bool, error)

	// Unlock освобождает блокировку
	Unlock(ctx context.Context, key string) error

	// UnlockWithTenant освобождает блокировку с учетом ID арендатора
	UnlockWithTenant(ctx context.Context, key string, tenantID string) error

	// Close закрывает соединение с системой кэширования
	Close() error
}
