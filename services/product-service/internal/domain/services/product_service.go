package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/athebyme/gomarket-platform/product-service/internal/adapters/storage"
	"github.com/athebyme/gomarket-platform/product-service/internal/domain/models"
	"github.com/google/uuid"
)

// ProductService предоставляет бизнес-логику для работы с продуктами
type ProductService struct {
	repository postgres.Port
}

// NewProductService создает новый экземпляр ProductService
func NewProductService(repository postgres.Repository) *ProductService {
	return &ProductService{
		repository: repository,
	}
}

// CreateProduct создает новый продукт
func (s *ProductService) CreateProduct(ctx context.Context, product *models.Product, tenantID string, userID string) (*models.Product, error) {
	// Генерируем ID для нового продукта, если ID не задан
	if product.ID == "" {
		product.ID = uuid.New().String()
	}

	// Устанавливаем время создания и обновления
	now := time.Now().UTC()
	product.CreatedAt = now
	product.UpdatedAt = now

	// Начинаем транзакцию
	txCtx, err := s.repository.(postgres.Port).BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Сохраняем продукт
	err = s.repository.SaveProduct(txCtx, product, tenantID)
	if err != nil {
		s.repository.(postgres.Port).RollbackTx(txCtx)
		return nil, fmt.Errorf("failed to save product: %w", err)
	}

	// Создаем запись в истории изменений
	historyRecord := &models.ProductHistoryRecord{
		ID:         uuid.New().String(),
		ProductID:  product.ID,
		ChangeType: "create",
		After:      product,
		ChangedBy:  userID,
		ChangedAt:  time.Now().Unix(),
	}

	err = s.repository.SaveHistoryRecord(txCtx, historyRecord, tenantID)
	if err != nil {
		s.repository.(postgres.Port).RollbackTx(txCtx)
		return nil, fmt.Errorf("failed to save history record: %w", err)
	}

	// Фиксируем транзакцию
	err = s.repository.(postgres.Port).CommitTx(txCtx)
	if err != nil {
		s.repository.(postgres.Port).RollbackTx(txCtx)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return product, nil
}

// UpdateProduct обновляет существующий продукт
func (s *ProductService) UpdateProduct(ctx context.Context, product *models.Product, tenantID string, userID string, comment string) (*models.Product, error) {
	// Получаем текущую версию продукта
	existingProduct, err := s.repository.GetProduct(ctx, product.ID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing product: %w", err)
	}

	if existingProduct == nil {
		return nil, errors.New("product not found")
	}

	// Устанавливаем время обновления
	now := time.Now().UTC()
	product.CreatedAt = existingProduct.CreatedAt
	product.UpdatedAt = now

	// Начинаем транзакцию
	txCtx, err := s.repository.(postgres.Port).BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Сохраняем обновленный продукт
	err = s.repository.SaveProduct(txCtx, product, tenantID)
	if err != nil {
		s.repository.(postgres.Port).RollbackTx(txCtx)
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	// Создаем запись в истории изменений
	historyRecord := &models.ProductHistoryRecord{
		ID:            uuid.New().String(),
		ProductID:     product.ID,
		ChangeType:    "update",
		Before:        existingProduct,
		After:         product,
		ChangedBy:     userID,
		ChangedAt:     time.Now().Unix(),
		ChangeComment: comment,
	}

	err = s.repository.SaveHistoryRecord(txCtx, historyRecord, tenantID)
	if err != nil {
		s.repository.(postgres.Port).RollbackTx(txCtx)
		return nil, fmt.Errorf("failed to save history record: %w", err)
	}

	// Фиксируем транзакцию
	err = s.repository.(postgres.Port).CommitTx(txCtx)
	if err != nil {
		s.repository.(postgres.Port).RollbackTx(txCtx)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return product, nil
}

// GetProduct получает продукт по ID
func (s *ProductService) GetProduct(ctx context.Context, productID string, tenantID string) (*models.Product, error) {
	product, err := s.repository.GetProduct(ctx, productID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	if product == nil {
		return nil, nil // Продукт не найден
	}

	return product, nil
}

// DeleteProduct удаляет продукт
func (s *ProductService) DeleteProduct(ctx context.Context, productID string, tenantID string, userID string, comment string) error {
	// Получаем текущую версию продукта для истории
	existingProduct, err := s.repository.GetProduct(ctx, productID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get existing product: %w", err)
	}

	if existingProduct == nil {
		return errors.New("product not found")
	}

	// Начинаем транзакцию
	txCtx, err := s.repository.(postgres.Port).BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Удаляем продукт
	err = s.repository.DeleteProduct(txCtx, productID, tenantID)
	if err != nil {
		s.repository.(postgres.Port).RollbackTx(txCtx)
		return fmt.Errorf("failed to delete product: %w", err)
	}

	// Создаем запись в истории изменений
	historyRecord := &models.ProductHistoryRecord{
		ID:            uuid.New().String(),
		ProductID:     productID,
		ChangeType:    "delete",
		Before:        existingProduct,
		ChangedBy:     userID,
		ChangedAt:     time.Now().Unix(),
		ChangeComment: comment,
	}

	err = s.repository.SaveHistoryRecord(txCtx, historyRecord, tenantID)
	if err != nil {
		s.repository.(postgres.Port).RollbackTx(txCtx)
		return fmt.Errorf("failed to save history record: %w", err)
	}

	// Фиксируем транзакцию
	err = s.repository.(postgres.Port).CommitTx(txCtx)
	if err != nil {
		s.repository.(postgres.Port).RollbackTx(txCtx)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ListProducts получает список продуктов с фильтрацией и пагинацией
func (s *ProductService) ListProducts(ctx context.Context, tenantID string, filter *models.ProductFilter, page, pageSize int) ([]*models.Product, int, error) {
	// Преобразуем фильтр в map для использования в репозитории
	filterMap := filter.ToMap()

	// Получаем список продуктов
	products, total, err := s.repository.ListProducts(ctx, tenantID, filterMap, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list products: %w", err)
	}

	return products, total, nil
}

// UpdateInventory обновляет информацию об инвентаре продукта
func (s *ProductService) UpdateInventory(ctx context.Context, inventory *models.ProductInventory, tenantID string) error {
	// Проверяем существование продукта
	product, err := s.repository.GetProduct(ctx, inventory.ProductID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}

	if product == nil {
		return errors.New("product not found")
	}

	// Установка времени обновления
	inventory.UpdatedAt = time.Now().UTC()

	// Сохраняем информацию об инвентаре
	err = s.repository.SaveInventory(ctx, inventory, tenantID)
	if err != nil {
		return fmt.Errorf("failed to update inventory: %w", err)
	}

	return nil
}

// UpdatePrice обновляет информацию о цене продукта
func (s *ProductService) UpdatePrice(ctx context.Context, price *models.ProductPrice, tenantID string) error {
	// Проверяем существование продукта
	product, err := s.repository.GetProduct(ctx, price.ProductID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}

	if product == nil {
		return errors.New("product not found")
	}

	// Установка времени обновления
	price.UpdatedAt = time.Now().UTC()

	// Сохраняем информацию о цене
	err = s.repository.SavePrice(ctx, price, tenantID)
	if err != nil {
		return fmt.Errorf("failed to update price: %w", err)
	}

	return nil
}

// AddMedia добавляет медиафайл к продукту
func (s *ProductService) AddMedia(ctx context.Context, media *models.ProductMedia, tenantID string) error {
	// Проверяем существование продукта
	product, err := s.repository.GetProduct(ctx, media.ProductID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}

	if product == nil {
		return errors.New("product not found")
	}

	// Генерируем ID для медиафайла, если не задан
	if media.ID == "" {
		media.ID = uuid.New().String()
	}

	// Установка времени создания
	media.CreatedAt = time.Now().UTC()

	// Сохраняем медиафайл
	err = s.repository.SaveMedia(ctx, media, tenantID)
	if err != nil {
		return fmt.Errorf("failed to add media: %w", err)
	}

	return nil
}

// GetProductWithRelations получает продукт со всеми связанными данными (цена, инвентарь, медиа)
func (s *ProductService) GetProductWithRelations(ctx context.Context, productID string, tenantID string) (map[string]interface{}, error) {
	// Получаем основные данные продукта
	product, err := s.repository.GetProduct(ctx, productID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	if product == nil {
		return nil, errors.New("product not found")
	}

	// Получаем данные об инвентаре
	inventory, err := s.repository.GetInventory(ctx, productID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}

	// Получаем данные о цене
	price, err := s.repository.GetPrice(ctx, productID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get price: %w", err)
	}

	// Получаем медиафайлы
	media, err := s.repository.GetMediaByProductID(ctx, productID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get media: %w", err)
	}

	// Преобразуем BaseData из JSON в map
	var baseData map[string]interface{}
	if len(product.BaseData) > 0 {
		err = json.Unmarshal(product.BaseData, &baseData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal base data: %w", err)
		}
	}

	// Преобразуем Metadata из JSON в map
	var metadata map[string]interface{}
	if len(product.Metadata) > 0 {
		err = json.Unmarshal(product.Metadata, &metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	// Формируем результат
	result := map[string]interface{}{
		"id":          product.ID,
		"supplier_id": product.SupplierID,
		"created_at":  product.CreatedAt,
		"updated_at":  product.UpdatedAt,
		"base_data":   baseData,
		"metadata":    metadata,
	}

	// Добавляем данные об инвентаре, если есть
	if inventory != nil {
		result["inventory"] = inventory
	}

	// Добавляем данные о цене, если есть
	if price != nil {
		result["price"] = price
	}

	// Добавляем медиафайлы, если есть
	if len(media) > 0 {
		result["media"] = media
	}

	return result, nil
}

// CreateCategory создает новую категорию продуктов
func (s *ProductService) CreateCategory(ctx context.Context, category *models.ProductCategory, tenantID string) (*models.ProductCategory, error) {
	// Генерируем ID для новой категории, если не задан
	if category.ID == "" {
		category.ID = uuid.New().String()
	}

	// Если это не корневая категория, проверяем существование родительской категории
	if category.ParentID != "" {
		parentCategory, err := s.repository.GetCategory(ctx, category.ParentID, tenantID)
		if err != nil {
			return nil, fmt.Errorf("failed to get parent category: %w", err)
		}

		if parentCategory == nil {
			return nil, errors.New("parent category not found")
		}

		// Устанавливаем уровень и путь
		category.Level = parentCategory.Level + 1
		category.Path = parentCategory.Path + "/" + category.ID
	} else {
		// Корневая категория
		category.Level = 1
		category.Path = "/" + category.ID
	}

	// Сохраняем категорию
	err := s.repository.SaveCategory(ctx, category, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to save category: %w", err)
	}

	return category, nil
}

// GetCategoryTree получает дерево категорий, начиная с указанной (или корневых категорий)
func (s *ProductService) GetCategoryTree(ctx context.Context, tenantID string, parentID string, maxDepth int) ([]*models.ProductCategory, error) {
	// Получаем категории первого уровня
	categories, err := s.repository.ListCategories(ctx, tenantID, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}

	// Если достигли максимальной глубины или нет подкатегорий, возвращаем результат
	if maxDepth <= 1 || len(categories) == 0 {
		return categories, nil
	}

	// Рекурсивно получаем подкатегории
	for _, category := range categories {
		if len(category.SubCategories) > 0 {
			subCategories, err := s.GetCategoryTree(ctx, tenantID, category.ID, maxDepth-1)
			if err != nil {
				return nil, err
			}

			// Создаем карту для быстрого доступа к подкатегориям
			subCategoryMap := make(map[string]*models.ProductCategory, len(subCategories))
			for _, subCategory := range subCategories {
				subCategoryMap[subCategory.ID] = subCategory
			}

			// Заполняем подкатегории в правильном порядке из SubCategories
			var orderedSubCategories []*models.ProductCategory
			for _, subCategoryID := range category.SubCategories {
				if subCat, ok := subCategoryMap[subCategoryID]; ok {
					orderedSubCategories = append(orderedSubCategories, subCat)
				}
			}

			// Добавляем новые подкатегории, которых не было в SubCategories
			for _, subCategory := range subCategories {
				found := false
				for _, existingID := range category.SubCategories {
					if subCategory.ID == existingID {
						found = true
						break
					}
				}
				if !found {
					orderedSubCategories = append(orderedSubCategories, subCategory)
				}
			}
		}
	}

	return categories, nil
}
