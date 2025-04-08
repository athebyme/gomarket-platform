package postgres

import (
	"context"
	"github.com/athebyme/gomarket-platform/product-service/internal/domain/models"
)

// Repository определяет интерфейс взаимодействия с хранилищем PostgreSQL
type Repository interface {
	// Product методы
	SaveProduct(ctx context.Context, product *models.Product, tenantID string) error
	GetProduct(ctx context.Context, productID string, tenantID string) (*models.Product, error)
	ListProducts(ctx context.Context, tenantID string, filters map[string]interface{}, page, pageSize int) ([]*models.Product, int, error)
	DeleteProduct(ctx context.Context, productID string, tenantID string) error

	// ProductInventory методы
	SaveInventory(ctx context.Context, inventory *models.ProductInventory, tenantID string) error
	GetInventory(ctx context.Context, productID string, tenantID string) (*models.ProductInventory, error)

	// ProductPrice методы
	SavePrice(ctx context.Context, price *models.ProductPrice, tenantID string) error
	GetPrice(ctx context.Context, productID string, tenantID string) (*models.ProductPrice, error)

	// ProductMedia методы
	SaveMedia(ctx context.Context, media *models.ProductMedia, tenantID string) error
	GetMediaByProductID(ctx context.Context, productID string, tenantID string) ([]*models.ProductMedia, error)
	DeleteMedia(ctx context.Context, mediaID string, tenantID string) error

	// ProductCategory методы
	SaveCategory(ctx context.Context, category *models.ProductCategory, tenantID string) error
	GetCategory(ctx context.Context, categoryID string, tenantID string) (*models.ProductCategory, error)
	ListCategories(ctx context.Context, tenantID string, parentID string) ([]*models.ProductCategory, error)
	DeleteCategory(ctx context.Context, categoryID string, tenantID string) error

	// ProductHistory методы
	SaveHistoryRecord(ctx context.Context, record *models.ProductHistoryRecord, tenantID string) error
	GetProductHistory(ctx context.Context, productID string, tenantID string, limit, offset int) ([]*models.ProductHistoryRecord, error)
}

type Port interface {
	Repository

	BeginTx(ctx context.Context) (context.Context, error)

	CommitTx(ctx context.Context) error

	RollbackTx(ctx context.Context) error

	Close() error
}
