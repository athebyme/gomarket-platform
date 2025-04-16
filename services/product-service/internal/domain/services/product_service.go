package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/athebyme/gomarket-platform/pkg/tx"
	"strings"
	"time"

	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/athebyme/gomarket-platform/product-service/internal/adapters/messaging"
	postgres "github.com/athebyme/gomarket-platform/product-service/internal/adapters/storage"
	"github.com/athebyme/gomarket-platform/product-service/internal/domain/models"
	"github.com/google/uuid"
)

type ProductServiceInterface interface {
	// Основные CRUD операции
	CreateProduct(ctx context.Context, product *models.Product) (*models.Product, error)
	GetProduct(ctx context.Context, productID, supplierID, tenantID string) (*models.Product, error)
	UpdateProduct(ctx context.Context, product *models.Product) (*models.Product, error)
	DeleteProduct(ctx context.Context, productID, supplierID, tenantID string) error
	ListProducts(ctx context.Context, tenantID string, filters map[string]interface{}, page, pageSize int) ([]*models.Product, int, error)

	// Операции с ценами и инвентарем
	UpdatePrice(ctx context.Context, price *models.ProductPrice, tenantID string) error
	UpdateInventory(ctx context.Context, inventory *models.ProductInventory, tenantID string) error

	// Синхронизация с внешними системами
	SyncProductToMarketplace(ctx context.Context, productID string, marketplaceID int, tenantID string) error
	SyncProductsFromSupplier(ctx context.Context, supplierID int, tenantID string) (int, error)

	// Кэширование
	InvalidateCache(ctx context.Context, key string, tenantID string) error
}

type ProductService struct {
	repository postgres.ProductStoragePort
	cache      interfaces.CachePort
	messaging  interfaces.MessagingPort
	logger     interfaces.LoggerPort
	txManager  tx.TxManager
}

// NewProductService создает новый экземпляр ProductService
func NewProductService(
	repo postgres.ProductStoragePort,
	cache interfaces.CachePort,
	msg interfaces.MessagingPort,
	log interfaces.LoggerPort,
	txMgr tx.TxManager,
) *ProductService {
	return &ProductService{
		repository: repo,
		cache:      cache,
		messaging:  msg,
		logger:     log,
		txManager:  txMgr,
	}
}

func (s *ProductService) CreateProduct(ctx context.Context, product *models.Product) (*models.Product, error) {
	var createdProduct *models.Product

	err := s.txManager.Do(ctx, func(txCtx context.Context) error {
		if product.ID == "" {
			product.ID = uuid.New().String()
		}
		now := time.Now().UTC()
		product.CreatedAt = now
		product.UpdatedAt = now

		if err := s.repository.SaveProduct(txCtx, product); err != nil {
			s.logger.ErrorWithContext(txCtx, "Ошибка сохранения продукта внутри транзакции",
				interfaces.LogField{Key: "error", Value: err},
				interfaces.LogField{Key: "product_id", Value: product.ID},
				interfaces.LogField{Key: "tenant_id", Value: product.TenantID},
			)
			return fmt.Errorf("repository.SaveProduct failed: %w", err)
		}

		createdProduct = product

		s.logger.InfoWithContext(txCtx, "Продукт успешно сохранен внутри транзакции", interfaces.LogField{Key: "product_id", Value: product.ID})
		return nil
	})

	if err != nil {
		s.logger.ErrorWithContext(ctx, "Ошибка выполнения транзакции создания продукта", interfaces.LogField{Key: "error", Value: err})
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	// ---- Транзакция успешно ЗАКОММИЧЕНА ----
	s.logger.InfoWithContext(ctx, "Транзакция создания продукта успешно закоммичена", interfaces.LogField{Key: "product_id", Value: createdProduct.ID})

	event := struct {
		EventType string                 `json:"event_type"`
		TenantID  string                 `json:"tenant_id"`
		Payload   map[string]interface{} `json:"payload"`
	}{
		EventType: messaging.ProductCreatedEvent,
		TenantID:  createdProduct.TenantID,
		Payload: map[string]interface{}{
			"product_id":  createdProduct.ID,
			"supplier_id": createdProduct.SupplierID,
		},
	}

	eventData, marshalErr := json.Marshal(event)
	if marshalErr != nil {
		s.logger.ErrorWithContext(ctx, "Ошибка маршалинга события ProductCreated после коммита",
			interfaces.LogField{Key: "error", Value: marshalErr},
			interfaces.LogField{Key: "product_id", Value: createdProduct.ID})
		// Продукт создан, но событие не уйдет. Логируем, но не возвращаем ошибку клиенту.
	} else {
		publishErr := s.messaging.Publish(ctx, "product-events", eventData)
		if publishErr != nil {
			s.logger.ErrorWithContext(ctx, "Ошибка публикации события ProductCreated после коммита",
				interfaces.LogField{Key: "error", Value: publishErr},
				interfaces.LogField{Key: "product_id", Value: createdProduct.ID})
			// ОЧЕНЬ ВАЖНО ЛОГИРОВАТЬ ЭТУ ОШИБКУ!
		} else {
			s.logger.InfoWithContext(ctx, "Событие ProductCreated успешно опубликовано после коммита",
				interfaces.LogField{Key: "product_id", Value: createdProduct.ID})
		}
	}

	return createdProduct, nil
}

func (s *ProductService) GetProduct(ctx context.Context, productID, supplierID, tenantID string) (*models.Product, error) {
	s.logger.DebugWithContext(ctx, "Запрос на получение продукта",
		interfaces.LogField{Key: "product_id", Value: productID},
		interfaces.LogField{Key: "supplier_id", Value: supplierID},
		interfaces.LogField{Key: "tenant_id", Value: tenantID},
	)

	cacheKey := fmt.Sprintf("product:%s:%s:%s", tenantID, supplierID, productID)

	cachedData, cacheErr := s.cache.GetWithTenant(ctx, cacheKey, tenantID)
	if cacheErr == nil && cachedData != nil {
		var product models.Product
		if unmarshalErr := json.Unmarshal(cachedData, &product); unmarshalErr == nil {
			s.logger.DebugWithContext(ctx, "Продукт получен из кэша",
				interfaces.LogField{Key: "product_id", Value: productID},
			)
			return &product, nil
		} else {
			s.logger.WarnWithContext(ctx, "Ошибка десериализации продукта из кэша",
				interfaces.LogField{Key: "error", Value: unmarshalErr.Error()},
			)
		}
	} else if cacheErr != nil && !errors.Is(cacheErr, interfaces.ErrCacheMiss) {
		s.logger.WarnWithContext(ctx, "Ошибка чтения из кэша",
			interfaces.LogField{Key: "error", Value: cacheErr.Error()},
		)
	}

	product, dbErr := s.repository.GetProductBySupplier(ctx, productID, supplierID, tenantID)
	if dbErr != nil {
		s.logger.ErrorWithContext(ctx, "Ошибка получения продукта из хранилища",
			interfaces.LogField{Key: "error", Value: dbErr.Error()},
			interfaces.LogField{Key: "product_id", Value: productID},
		)
		return nil, fmt.Errorf("failed to get product: %w", dbErr)
	}

	if product == nil {
		s.logger.InfoWithContext(ctx, "Продукт не найден",
			interfaces.LogField{Key: "product_id", Value: productID},
			interfaces.LogField{Key: "supplier_id", Value: supplierID},
		)
		return nil, nil
	}

	productJSON, marshalErr := json.Marshal(product)
	if marshalErr == nil {
		if cacheSetErr := s.cache.SetWithTenant(ctx, cacheKey, productJSON, tenantID, 30*time.Minute); cacheSetErr != nil {
			s.logger.WarnWithContext(ctx, "Ошибка сохранения продукта в кэш",
				interfaces.LogField{Key: "error", Value: cacheSetErr.Error()},
			)
		}
	} else {
		s.logger.WarnWithContext(ctx, "Ошибка сериализации продукта для кэша",
			interfaces.LogField{Key: "error", Value: marshalErr.Error()},
		)
	}

	s.logger.DebugWithContext(ctx, "Продукт успешно получен",
		interfaces.LogField{Key: "product_id", Value: productID},
	)

	return product, nil
}

func (s *ProductService) UpdateProduct(ctx context.Context, product *models.Product) (*models.Product, error) {
	if product.ID == "" || product.TenantID == "" {
		return nil, errors.New("product ID and tenant ID cannot be empty")
	}

	product.UpdatedAt = time.Now().UTC()

	err := s.repository.SaveProduct(ctx, product)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "Failed to update product",
			interfaces.LogField{Key: "error", Value: err.Error()},
			interfaces.LogField{Key: "product_id", Value: product.ID},
		)
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	cacheKey := fmt.Sprintf("product:%s:%s:%s", product.TenantID, product.SupplierID, product.ID)
	_ = s.cache.DeleteWithTenant(ctx, cacheKey, product.TenantID)

	event := struct {
		EventType string                 `json:"event_type"`
		TenantID  string                 `json:"tenant_id"`
		Payload   map[string]interface{} `json:"payload"`
	}{
		EventType: messaging.ProductUpdatedEvent,
		TenantID:  product.TenantID,
		Payload: map[string]interface{}{
			"product_id":  product.ID,
			"supplier_id": product.SupplierID,
		},
	}

	eventData, _ := json.Marshal(event)
	_ = s.messaging.Publish(ctx, "product-events", eventData)

	return product, nil
}

func (s *ProductService) DeleteProduct(ctx context.Context, productID, supplierID, tenantID string) error {
	if productID == "" || tenantID == "" {
		return errors.New("product ID and tenant ID cannot be empty")
	}

	err := s.repository.DeleteProduct(ctx, productID, tenantID)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "Failed to delete product",
			interfaces.LogField{Key: "error", Value: err.Error()},
			interfaces.LogField{Key: "product_id", Value: productID},
		)
		return fmt.Errorf("failed to delete product: %w", err)
	}

	cacheKey := fmt.Sprintf("product:%s:%s:%s", tenantID, supplierID, productID)
	_ = s.cache.DeleteWithTenant(ctx, cacheKey, tenantID)

	_ = s.cache.DeleteByPatternWithTenant(ctx, "products:list:*", tenantID)

	event := struct {
		EventType string                 `json:"event_type"`
		TenantID  string                 `json:"tenant_id"`
		Payload   map[string]interface{} `json:"payload"`
	}{
		EventType: messaging.ProductDeletedEvent,
		TenantID:  tenantID,
		Payload: map[string]interface{}{
			"product_id":  productID,
			"supplier_id": supplierID,
		},
	}

	eventData, _ := json.Marshal(event)
	_ = s.messaging.Publish(ctx, "product-events", eventData)

	return nil
}

func (s *ProductService) ListProducts(ctx context.Context, tenantID string, filters map[string]interface{}, page, pageSize int) ([]*models.Product, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	} else if pageSize > 100 {
		pageSize = 100
	}

	if len(filters) == 0 {
		cacheKey := fmt.Sprintf("products:list:%s:%d:%d", tenantID, page, pageSize)
		cachedData, err := s.cache.GetWithTenant(ctx, cacheKey, tenantID)

		if err == nil && cachedData != nil {
			var result struct {
				Products []*models.Product `json:"products"`
				Total    int               `json:"total"`
			}

			if err := json.Unmarshal(cachedData, &result); err == nil {
				return result.Products, result.Total, nil
			}
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	products, total, err := s.repository.ListProducts(ctx, tenantID, filters, page, pageSize)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "Failed to list products",
			interfaces.LogField{Key: "error", Value: err.Error()},
		)
		return nil, 0, fmt.Errorf("failed to list products: %w", err)
	}

	if len(filters) == 0 {
		cacheKey := fmt.Sprintf("products:list:%s:%d:%d", tenantID, page, pageSize)
		cacheData := struct {
			Products []*models.Product `json:"products"`
			Total    int               `json:"total"`
		}{
			Products: products,
			Total:    total,
		}

		if cacheJSON, err := json.Marshal(cacheData); err == nil {
			_ = s.cache.SetWithTenant(ctx, cacheKey, cacheJSON, tenantID, 5*time.Minute)
		}
	}

	return products, total, nil
}

func (s *ProductService) UpdatePrice(ctx context.Context, price *models.ProductPrice, tenantID string) error {
	price.UpdatedAt = time.Now().UTC()

	err := s.repository.SavePrice(ctx, price, tenantID)
	if err != nil {
		return fmt.Errorf("failed to save price: %w", err)
	}

	cacheKey := fmt.Sprintf("product:%s:%s:%s", tenantID, price.SupplierID, price.ProductID)
	_ = s.cache.DeleteWithTenant(ctx, cacheKey, tenantID)

	return nil
}

func (s *ProductService) UpdateInventory(ctx context.Context, inventory *models.ProductInventory, tenantID string) error {
	inventory.UpdatedAt = time.Now().UTC()

	err := s.repository.SaveInventory(ctx, inventory, tenantID)
	if err != nil {
		return fmt.Errorf("failed to save inventory: %w", err)
	}

	cacheKey := fmt.Sprintf("product:%s:%s:%s", tenantID, inventory.SupplierID, inventory.ProductID)
	_ = s.cache.DeleteWithTenant(ctx, cacheKey, tenantID)

	return nil
}

func (s *ProductService) SyncProductToMarketplace(ctx context.Context, productID string, marketplaceID int, tenantID string) error {
	product, err := s.repository.GetProduct(ctx, productID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}
	if product == nil {
		return fmt.Errorf("product not found: %s", productID)
	}

	event := struct {
		EventType     string    `json:"event_type"`
		TenantID      string    `json:"tenant_id"`
		ProductID     string    `json:"product_id"`
		MarketplaceID int       `json:"marketplace_id"`
		Timestamp     time.Time `json:"timestamp"`
	}{
		EventType:     "product_marketplace_sync",
		TenantID:      tenantID,
		ProductID:     productID,
		MarketplaceID: marketplaceID,
		Timestamp:     time.Now().UTC(),
	}

	eventData, _ := json.Marshal(event)
	return s.messaging.Publish(ctx, "marketplace-sync", eventData)
}

func (s *ProductService) SyncProductsFromSupplier(ctx context.Context, supplierID int, tenantID string) (int, error) {
	event := struct {
		EventType  string    `json:"event_type"`
		TenantID   string    `json:"tenant_id"`
		SupplierID int       `json:"supplier_id"`
		Timestamp  time.Time `json:"timestamp"`
	}{
		EventType:  "supplier_sync_requested",
		TenantID:   tenantID,
		SupplierID: supplierID,
		Timestamp:  time.Now().UTC(),
	}

	eventData, _ := json.Marshal(event)
	err := s.messaging.Publish(ctx, "supplier-sync", eventData)
	if err != nil {
		return 0, fmt.Errorf("failed to queue supplier sync: %w", err)
	}

	return 0, nil
}

func (s *ProductService) PublishProductEvent(ctx context.Context, productID string, eventType string) error {
	event := struct {
		EventType string    `json:"event_type"`
		TenantID  string    `json:"tenant_id"`
		ProductID string    `json:"product_id"`
		Timestamp time.Time `json:"timestamp"`
	}{
		EventType: eventType,
		TenantID:  ctx.Value("tenant_id").(string),
		ProductID: productID,
		Timestamp: time.Now().UTC(),
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("ошибка сериализации события: %w", err)
	}

	err = s.messaging.Publish(ctx, "product-events", eventData)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "Ошибка публикации события продукта",
			interfaces.LogField{Key: "event_type", Value: eventType},
			interfaces.LogField{Key: "product_id", Value: productID},
			interfaces.LogField{Key: "error", Value: err.Error()},
		)
		return fmt.Errorf("ошибка публикации события: %w", err)
	}

	s.logger.InfoWithContext(ctx, "Событие продукта опубликовано",
		interfaces.LogField{Key: "event_type", Value: eventType},
		interfaces.LogField{Key: "product_id", Value: productID},
	)

	return nil
}

func (s *ProductService) InvalidateCache(ctx context.Context, key string, tenantID string) error {
	if key == "" {
		pattern := fmt.Sprintf("tenant:%s:*", tenantID)
		return s.cache.DeleteByPattern(ctx, pattern)
	} else if strings.HasSuffix(key, "*") {
		return s.cache.DeleteByPatternWithTenant(ctx, key, tenantID)
	} else {
		return s.cache.DeleteWithTenant(ctx, key, tenantID)
	}
}
