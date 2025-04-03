package interfaces

import (
	"context"
)

// StoragePort определяет интерфейс для работы с постоянным хранилищем данных
// Реализация может использовать любую базу данных (PostgreSQL, MySQL, MongoDB и т.д.)
type StoragePort interface {
	// BeginTx начинает новую транзакцию
	BeginTx(ctx context.Context) (context.Context, error)

	// CommitTx фиксирует транзакцию
	CommitTx(ctx context.Context) error

	// RollbackTx откатывает транзакцию
	RollbackTx(ctx context.Context) error

	// Метод закрытия соединения

	// Close закрывает соединение с хранилищем
	Close() error
}

//// ExtendedStoragePort расширяет базовый StoragePort дополнительными методами
//type ExtendedStoragePort interface {
//	StoragePort
//
//	// Методы с улучшенной фильтрацией и пагинацией
//
//	// ListProductsWithFilter возвращает список продуктов с учетом структурированного фильтра и пагинации
//	ListProductsWithFilter(ctx context.Context, tenantID string, filter *models.ProductFilter, pagination *models.Pagination) (*models.PagedResult, error)
//
//	// CountProducts возвращает количество продуктов с учетом фильтра
//	CountProducts(ctx context.Context, tenantID string, filter *models.ProductFilter) (int64, error)
//
//	// Методы для работы с историей изменений
//
//	// SaveProductHistory сохраняет историю изменений продукта
//	SaveProductHistory(ctx context.Context, productID string, changeType string, before, after *models.Product, tenantID string) error
//
//	// GetProductHistory возвращает историю изменений продукта
//	GetProductHistory(ctx context.Context, productID string, tenantID string, pagination *models.Pagination) (*models.PagedResult, error)
//
//	// Методы для работы с групповыми операциями
//
//	// BatchSaveProducts сохраняет несколько продуктов за одну операцию
//	BatchSaveProducts(ctx context.Context, products []*models.Product, tenantID string) error
//
//	// BatchDeleteProducts удаляет несколько продуктов за одну операцию
//	BatchDeleteProducts(ctx context.Context, productIDs []string, tenantID string) error
//
//	// Методы для работы с категориями продуктов
//
//	// GetProductCategories возвращает все категории продуктов
//	GetProductCategories(ctx context.Context, tenantID string) ([]*models.ProductCategory, error)
//
//	// GetProductsByCategory возвращает продукты из указанной категории
//	GetProductsByCategory(ctx context.Context, categoryID string, tenantID string, pagination *models.Pagination) (*models.PagedResult, error)
//
//	// Методы для поиска
//
//	// SearchProducts выполняет полнотекстовый поиск по продуктам
//	SearchProducts(ctx context.Context, query string, tenantID string, pagination *models.Pagination) (*models.PagedResult, error)
//
//	// Методы для работы с связанными продуктами
//
//	// GetRelatedProducts возвращает связанные продукты
//	GetRelatedProducts(ctx context.Context, productID string, tenantID string, limit int) ([]*models.Product, error)
//
//	// SaveRelatedProducts сохраняет связи между продуктами
//	SaveRelatedProducts(ctx context.Context, productID string, relatedIDs []string, tenantID string) error
//}
