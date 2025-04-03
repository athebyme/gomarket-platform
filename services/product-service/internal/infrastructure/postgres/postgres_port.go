package postgres

import (
	"context"
	"github.com/athebyme/gomarket-platform/product-service/internal/domain/models"
)

type Storage interface {
	// Методы для работы с продуктами

	// SaveProduct сохраняет продукт в хранилище
	// Если продукт с таким ID уже существует, он будет обновлен
	SaveProduct(ctx context.Context, product *models.Product, tenantID string) error

	// GetProduct получает продукт по ID
	// Возвращает nil, nil если продукт не найден
	GetProduct(ctx context.Context, productID string, tenantID string) (*models.Product, error)

	// ListProducts возвращает список продуктов с поддержкой пагинации и фильтрации
	ListProducts(ctx context.Context, tenantID string, filters map[string]interface{}, page, pageSize int) ([]*models.Product, int, error)

	// DeleteProduct удаляет продукт из хранилища
	DeleteProduct(ctx context.Context, productID string, tenantID string) error
}

type Port interface {
	Storage

	BeginTx(ctx context.Context) (context.Context, error)

	CommitTx(ctx context.Context) error

	RollbackTx(ctx context.Context) error

	Close() error
}
