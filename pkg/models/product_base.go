package models

import (
	"context"
	"github.com/athebyme/gomarket-platform/pkg/dto"
)

// Service определяет интерфейс сервиса для работы с продуктами
type Service interface {
	// GetProduct получает продукт по ID с учетом арендатора
	GetProduct(ctx context.Context, productID string, tenantID string) (*dto.ProductDTO, error)

	// CreateProduct создает новый продукт
	CreateProduct(ctx context.Context, product *dto.ProductDTO, tenantID string) (*dto.ProductDTO, error)

	// UpdateProduct обновляет существующий продукт
	UpdateProduct(ctx context.Context, product *dto.ProductDTO, tenantID string) (*dto.ProductDTO, error)

	// DeleteProduct удаляет продукт
	DeleteProduct(ctx context.Context, productID string, tenantID string) error

	// ListProducts возвращает список продуктов с поддержкой пагинации и фильтрации
	ListProducts(ctx context.Context, filters map[string]interface{}, page, pageSize int, tenantID string) ([]*dto.ProductDTO, int, error)

	// SyncProductsFromSupplier синхронизирует продукты от поставщика
	SyncProductsFromSupplier(ctx context.Context, supplierID int, tenantID string) (int, error)

	// SyncProductToMarketplace синхронизирует продукт с маркетплейсом
	SyncProductToMarketplace(ctx context.Context, productID string, marketplaceID int, tenantID string) error
}

//
//// ExtendedService расширяет базовый интерфейс ProductService
//type ExtendedService interface {
//	Service
//
//	// Методы с улучшенной фильтрацией и пагинацией
//
//	// ListProductsWithFilter возвращает список продуктов с учетом структурированного фильтра и пагинации
//	ListProductsWithFilter(ctx context.Context, filter *models.ProductFilter, pagination *models.Pagination, tenantID string) (*models.PagedResult, error)
//
//	// Методы для массовых операций
//
//	// BatchCreateProducts создает несколько продуктов за одну операцию
//	BatchCreateProducts(ctx context.Context, products []*models.Product, tenantID string) (int, error)
//
//	// BatchUpdateProducts обновляет несколько продуктов за одну операцию
//	BatchUpdateProducts(ctx context.Context, products []*models.Product, tenantID string) (int, error)
//
//	// BatchDeleteProducts удаляет несколько продуктов за одну операцию
//	BatchDeleteProducts(ctx context.Context, productIDs []string, tenantID string) (int, error)
//
//	// Методы для работы с историей изменений
//
//	// GetProductHistory возвращает историю изменений продукта
//	GetProductHistory(ctx context.Context, productID string, pagination *models.Pagination, tenantID string) (*models.PagedResult, error)
//
//	// Методы для работы с категориями
//
//	// GetProductCategories возвращает все категории продуктов
//	GetProductCategories(ctx context.Context, tenantID string) ([]*models.ProductCategory, error)
//
//	// GetProductsByCategory возвращает продукты из указанной категории
//	GetProductsByCategory(ctx context.Context, categoryID string, pagination *models.Pagination, tenantID string) (*models.PagedResult, error)
//
//	// Методы для полнотекстового поиска
//
//	// SearchProducts выполняет полнотекстовый поиск по продуктам
//	SearchProducts(ctx context.Context, query string, pagination *models.Pagination, tenantID string) (*models.PagedResult, error)
//
//	// Методы для работы с связанными продуктами
//
//	// GetRelatedProducts возвращает связанные продукты
//	GetRelatedProducts(ctx context.Context, productID string, limit int, tenantID string) ([]*models.Product, error)
//
//	// SetRelatedProducts устанавливает связи между продуктами
//	SetRelatedProducts(ctx context.Context, productID string, relatedIDs []string, tenantID string) error
//}
