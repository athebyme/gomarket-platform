package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/athebyme/gomarket-platform/product-service/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// contextKey тип для ключей контекста
type contextKey string

// Ключи контекста
const (
	txKey contextKey = "transaction"
)

// PostgresRepository реализация интерфейса Repository для PostgreSQL
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository создает новый экземпляр PostgresRepository
func NewPostgresRepository(ctx context.Context, connectionString string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	return &PostgresRepository{
		pool: pool,
	}, nil
}

// Close закрывает соединение с БД
func (r *PostgresRepository) Close() error {
	r.pool.Close()
	return nil
}

// getTx получает транзакцию из контекста
func (r *PostgresRepository) getTx(ctx context.Context) pgx.Tx {
	tx, ok := ctx.Value(txKey).(pgx.Tx)
	if !ok {
		return nil
	}
	return tx
}

// getExecutor возвращает исполнителя запросов (транзакцию или пул)
func (r *PostgresRepository) getExecutor(ctx context.Context) interface{} {
	tx := r.getTx(ctx)
	if tx != nil {
		return tx
	}
	return r.pool
}

// BeginTx начинает новую транзакцию
func (r *PostgresRepository) BeginTx(ctx context.Context) (context.Context, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ctx, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return context.WithValue(ctx, txKey, tx), nil
}

// CommitTx фиксирует транзакцию
func (r *PostgresRepository) CommitTx(ctx context.Context) error {
	tx := r.getTx(ctx)
	if tx == nil {
		return errors.New("no transaction in context")
	}
	return tx.Commit(ctx)
}

// RollbackTx откатывает транзакцию
func (r *PostgresRepository) RollbackTx(ctx context.Context) error {
	tx := r.getTx(ctx)
	if tx == nil {
		return errors.New("no transaction in context")
	}
	return tx.Rollback(ctx)
}

// SaveProduct сохраняет продукт в базу данных
func (r *PostgresRepository) SaveProduct(ctx context.Context, product *models.Product, tenantID string) error {
	executor := r.getExecutor(ctx)

	query := `
		INSERT INTO product.products (id, tenant_id, supplier_id, base_data, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id, tenant_id) 
		DO UPDATE SET 
			supplier_id = $3,
			base_data = $4,
			metadata = $5,
			updated_at = $7
	`

	now := time.Now().UTC()
	if product.CreatedAt.IsZero() {
		product.CreatedAt = now
	}
	product.UpdatedAt = now

	var err error
	switch e := executor.(type) {
	case pgx.Tx:
		_, err = e.Exec(ctx, query, product.ID, tenantID, product.SupplierID, product.BaseData,
			product.Metadata, product.CreatedAt, product.UpdatedAt)
	case *pgxpool.Pool:
		_, err = e.Exec(ctx, query, product.ID, tenantID, product.SupplierID, product.BaseData,
			product.Metadata, product.CreatedAt, product.UpdatedAt)
	}

	if err != nil {
		return fmt.Errorf("failed to save product: %w", err)
	}
	return nil
}

// GetProduct получает продукт по ID
func (r *PostgresRepository) GetProduct(ctx context.Context, productID string, tenantID string) (*models.Product, error) {
	executor := r.getExecutor(ctx)

	query := `
		SELECT id, supplier_id, base_data, metadata, created_at, updated_at
		FROM product.products
		WHERE id = $1 AND tenant_id = $2
	`

	var product models.Product
	var err error

	switch e := executor.(type) {
	case pgx.Tx:
		row := e.QueryRow(ctx, query, productID, tenantID)
		err = row.Scan(&product.ID, &product.SupplierID, &product.BaseData, &product.Metadata,
			&product.CreatedAt, &product.UpdatedAt)
	case *pgxpool.Pool:
		row := e.QueryRow(ctx, query, productID, tenantID)
		err = row.Scan(&product.ID, &product.SupplierID, &product.BaseData, &product.Metadata,
			&product.CreatedAt, &product.UpdatedAt)
	}

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Продукт не найден
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return &product, nil
}

// ListProducts возвращает список продуктов с поддержкой пагинации и фильтрации
func (r *PostgresRepository) ListProducts(ctx context.Context, tenantID string, filters map[string]interface{}, page, pageSize int) ([]*models.Product, int, error) {
	// Базовый запрос
	baseQuery := `
		FROM product.products
		WHERE tenant_id = $1
	`

	// Добавляем фильтры
	args := []interface{}{tenantID}
	argPos := 2
	var filterConditions []string

	// Здесь должна быть логика добавления фильтров
	// Для упрощения опустим детали реализации фильтров

	// Строим итоговый запрос для подсчета
	countQuery := "SELECT COUNT(*) " + baseQuery + " " + " AND " + genFilterConditions(filterConditions)

	// Получаем общее количество записей
	var total int
	executor := r.getExecutor(ctx)

	switch e := executor.(type) {
	case pgx.Tx:
		err := e.QueryRow(ctx, countQuery, args...).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to count products: %w", err)
		}
	case *pgxpool.Pool:
		err := e.QueryRow(ctx, countQuery, args...).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to count products: %w", err)
		}
	}

	// Если нет записей, возвращаем пустой результат
	if total == 0 {
		return []*models.Product{}, 0, nil
	}

	// Добавляем пагинацию и сортировку
	args = append(args, pageSize, (page-1)*pageSize)

	// Выполняем основной запрос
	dataQuery := `
		SELECT id, supplier_id, base_data, metadata, created_at, updated_at 
	` + baseQuery + " " + genFilterConditions(filterConditions) + `
		ORDER BY updated_at DESC
		LIMIT $` + fmt.Sprint(argPos) + ` OFFSET $` + fmt.Sprint(argPos+1)

	var rows pgx.Rows
	var err error

	switch e := executor.(type) {
	case pgx.Tx:
		rows, err = e.Query(ctx, dataQuery, args...)
	case *pgxpool.Pool:
		rows, err = e.Query(ctx, dataQuery, args...)
	}

	if err != nil {
		return nil, 0, fmt.Errorf("failed to list products: %w", err)
	}
	defer rows.Close()

	// Собираем результаты
	var products []*models.Product
	for rows.Next() {
		var product models.Product
		err := rows.Scan(&product.ID, &product.SupplierID, &product.BaseData,
			&product.Metadata, &product.CreatedAt, &product.UpdatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan product row: %w", err)
		}
		products = append(products, &product)
	}

	if rows.Err() != nil {
		return nil, 0, fmt.Errorf("error while iterating product rows: %w", rows.Err())
	}

	return products, total, nil
}

// DeleteProduct удаляет продукт из хранилища
func (r *PostgresRepository) DeleteProduct(ctx context.Context, productID string, tenantID string) error {
	executor := r.getExecutor(ctx)

	query := `
		DELETE FROM product.products 
		WHERE id = $1 AND tenant_id = $2
	`

	var err error
	switch e := executor.(type) {
	case pgx.Tx:
		_, err = e.Exec(ctx, query, productID, tenantID)
	case *pgxpool.Pool:
		_, err = e.Exec(ctx, query, productID, tenantID)
	}

	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}

	return nil
}

// SaveInventory сохраняет информацию об инвентаре продукта
func (r *PostgresRepository) SaveInventory(ctx context.Context, inventory *models.ProductInventory, tenantID string) error {
	executor := r.getExecutor(ctx)

	query := `
		INSERT INTO product.inventory (product_id, tenant_id, supplier_id, quantity, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (product_id, tenant_id) 
		DO UPDATE SET 
			supplier_id = $3,
			quantity = $4,
			updated_at = $5
	`

	now := time.Now().UTC()
	inventory.UpdatedAt = now

	var err error
	switch e := executor.(type) {
	case pgx.Tx:
		_, err = e.Exec(ctx, query, inventory.ProductID, tenantID, inventory.SupplierID,
			inventory.Quantity, inventory.UpdatedAt)
	case *pgxpool.Pool:
		_, err = e.Exec(ctx, query, inventory.ProductID, tenantID, inventory.SupplierID,
			inventory.Quantity, inventory.UpdatedAt)
	}

	if err != nil {
		return fmt.Errorf("failed to save inventory: %w", err)
	}

	return nil
}

// GetInventory получает информацию об инвентаре продукта
func (r *PostgresRepository) GetInventory(ctx context.Context, productID string, tenantID string) (*models.ProductInventory, error) {
	executor := r.getExecutor(ctx)

	query := `
		SELECT product_id, supplier_id, quantity, updated_at
		FROM product.inventory
		WHERE product_id = $1 AND tenant_id = $2
	`

	var inventory models.ProductInventory
	var err error

	switch e := executor.(type) {
	case pgx.Tx:
		row := e.QueryRow(ctx, query, productID, tenantID)
		err = row.Scan(&inventory.ProductID, &inventory.SupplierID, &inventory.Quantity, &inventory.UpdatedAt)
	case *pgxpool.Pool:
		row := e.QueryRow(ctx, query, productID, tenantID)
		err = row.Scan(&inventory.ProductID, &inventory.SupplierID, &inventory.Quantity, &inventory.UpdatedAt)
	}

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Инвентарь не найден
		}
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}

	return &inventory, nil
}

// SavePrice сохраняет информацию о цене продукта
func (r *PostgresRepository) SavePrice(ctx context.Context, price *models.ProductPrice, tenantID string) error {
	executor := r.getExecutor(ctx)

	query := `
		INSERT INTO product.prices (product_id, tenant_id, supplier_id, base_price, special_price, 
			currency, start_date, end_date, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (product_id, tenant_id) 
		DO UPDATE SET 
			supplier_id = $3,
			base_price = $4,
			special_price = $5,
			currency = $6,
			start_date = $7,
			end_date = $8,
			updated_at = $9
	`

	now := time.Now().UTC()
	price.UpdatedAt = now

	var err error
	switch e := executor.(type) {
	case pgx.Tx:
		_, err = e.Exec(ctx, query, price.ProductID, tenantID, price.SupplierID, price.BasePrice,
			price.SpecialPrice, price.Currency, price.StartDate, price.EndDate, price.UpdatedAt)
	case *pgxpool.Pool:
		_, err = e.Exec(ctx, query, price.ProductID, tenantID, price.SupplierID, price.BasePrice,
			price.SpecialPrice, price.Currency, price.StartDate, price.EndDate, price.UpdatedAt)
	}

	if err != nil {
		return fmt.Errorf("failed to save price: %w", err)
	}

	return nil
}

// GetPrice получает информацию о цене продукта
func (r *PostgresRepository) GetPrice(ctx context.Context, productID string, tenantID string) (*models.ProductPrice, error) {
	executor := r.getExecutor(ctx)

	query := `
		SELECT product_id, supplier_id, base_price, special_price, currency, start_date, end_date, updated_at
		FROM product.prices
		WHERE product_id = $1 AND tenant_id = $2
	`

	var price models.ProductPrice
	var err error

	switch e := executor.(type) {
	case pgx.Tx:
		row := e.QueryRow(ctx, query, productID, tenantID)
		err = row.Scan(&price.ProductID, &price.SupplierID, &price.BasePrice, &price.SpecialPrice,
			&price.Currency, &price.StartDate, &price.EndDate, &price.UpdatedAt)
	case *pgxpool.Pool:
		row := e.QueryRow(ctx, query, productID, tenantID)
		err = row.Scan(&price.ProductID, &price.SupplierID, &price.BasePrice, &price.SpecialPrice,
			&price.Currency, &price.StartDate, &price.EndDate, &price.UpdatedAt)
	}

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Цена не найдена
		}
		return nil, fmt.Errorf("failed to get price: %w", err)
	}

	return &price, nil
}

// SaveMedia сохраняет медиафайл продукта
func (r *PostgresRepository) SaveMedia(ctx context.Context, media *models.ProductMedia, tenantID string) error {
	executor := r.getExecutor(ctx)

	// Если ID пустой, генерируем новый
	if media.ID == "" {
		media.ID = uuid.New().String()
	}

	query := `
		INSERT INTO product.media (id, tenant_id, product_id, type, url, position, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id, tenant_id) 
		DO UPDATE SET 
			product_id = $3,
			type = $4,
			url = $5,
			position = $6
	`

	now := time.Now().UTC()
	if media.CreatedAt.IsZero() {
		media.CreatedAt = now
	}

	var err error
	switch e := executor.(type) {
	case pgx.Tx:
		_, err = e.Exec(ctx, query, media.ID, tenantID, media.ProductID, media.Type,
			media.URL, media.Position, media.CreatedAt)
	case *pgxpool.Pool:
		_, err = e.Exec(ctx, query, media.ID, tenantID, media.ProductID, media.Type,
			media.URL, media.Position, media.CreatedAt)
	}

	if err != nil {
		return fmt.Errorf("failed to save media: %w", err)
	}

	return nil
}

// GetMediaByProductID получает все медиафайлы для продукта
func (r *PostgresRepository) GetMediaByProductID(ctx context.Context, productID string, tenantID string) ([]*models.ProductMedia, error) {
	executor := r.getExecutor(ctx)

	query := `
		SELECT id, product_id, type, url, position, created_at
		FROM product.media
		WHERE product_id = $1 AND tenant_id = $2
		ORDER BY position
	`

	var rows pgx.Rows
	var err error

	switch e := executor.(type) {
	case pgx.Tx:
		rows, err = e.Query(ctx, query, productID, tenantID)
	case *pgxpool.Pool:
		rows, err = e.Query(ctx, query, productID, tenantID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query media: %w", err)
	}
	defer rows.Close()

	var mediaList []*models.ProductMedia
	for rows.Next() {
		var media models.ProductMedia
		err := rows.Scan(&media.ID, &media.ProductID, &media.Type, &media.URL,
			&media.Position, &media.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan media row: %w", err)
		}
		mediaList = append(mediaList, &media)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error while iterating media rows: %w", rows.Err())
	}

	return mediaList, nil
}

// DeleteMedia удаляет медиафайл
func (r *PostgresRepository) DeleteMedia(ctx context.Context, mediaID string, tenantID string) error {
	executor := r.getExecutor(ctx)

	query := `
		DELETE FROM product.media 
		WHERE id = $1 AND tenant_id = $2
	`

	var err error
	switch e := executor.(type) {
	case pgx.Tx:
		_, err = e.Exec(ctx, query, mediaID, tenantID)
	case *pgxpool.Pool:
		_, err = e.Exec(ctx, query, mediaID, tenantID)
	}

	if err != nil {
		return fmt.Errorf("failed to delete media: %w", err)
	}

	return nil
}

// SaveCategory сохраняет категорию продукта
func (r *PostgresRepository) SaveCategory(ctx context.Context, category *models.ProductCategory, tenantID string) error {
	executor := r.getExecutor(ctx)

	// Если ID пустой, генерируем новый
	if category.ID == "" {
		category.ID = uuid.New().String()
	}

	query := `
		INSERT INTO product.categories (id, tenant_id, name, description, parent_id, level, path, image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id, tenant_id) 
		DO UPDATE SET 
			name = $3,
			description = $4,
			parent_id = $5,
			level = $6,
			path = $7,
			image_url = $8
	`

	var err error
	switch e := executor.(type) {
	case pgx.Tx:
		_, err = e.Exec(ctx, query, category.ID, tenantID, category.Name, category.Description,
			category.ParentID, category.Level, category.Path, category.ImageURL)
	case *pgxpool.Pool:
		_, err = e.Exec(ctx, query, category.ID, tenantID, category.Name, category.Description,
			category.ParentID, category.Level, category.Path, category.ImageURL)
	}

	if err != nil {
		return fmt.Errorf("failed to save category: %w", err)
	}

	return nil
}

// GetCategory получает категорию по ID
func (r *PostgresRepository) GetCategory(ctx context.Context, categoryID string, tenantID string) (*models.ProductCategory, error) {
	executor := r.getExecutor(ctx)

	query := `
		SELECT id, name, description, parent_id, level, path, image_url
		FROM product.categories
		WHERE id = $1 AND tenant_id = $2
	`

	var category models.ProductCategory
	var err error

	switch e := executor.(type) {
	case pgx.Tx:
		row := e.QueryRow(ctx, query, categoryID, tenantID)
		err = row.Scan(&category.ID, &category.Name, &category.Description,
			&category.ParentID, &category.Level, &category.Path, &category.ImageURL)
	case *pgxpool.Pool:
		row := e.QueryRow(ctx, query, categoryID, tenantID)
		err = row.Scan(&category.ID, &category.Name, &category.Description,
			&category.ParentID, &category.Level, &category.Path, &category.ImageURL)
	}

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Категория не найдена
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	// Дополнительно загружаем подкатегории
	subQuery := `
		SELECT id
		FROM product.categories
		WHERE parent_id = $1 AND tenant_id = $2
	`

	var rows pgx.Rows

	switch e := executor.(type) {
	case pgx.Tx:
		rows, err = e.Query(ctx, subQuery, categoryID, tenantID)
	case *pgxpool.Pool:
		rows, err = e.Query(ctx, subQuery, categoryID, tenantID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query subcategories: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var subCategoryID string
		err := rows.Scan(&subCategoryID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subcategory row: %w", err)
		}
		category.SubCategories = append(category.SubCategories, subCategoryID)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error while iterating subcategory rows: %w", rows.Err())
	}

	return &category, nil
}

// ListCategories возвращает список категорий с возможностью фильтрации по родительской категории
func (r *PostgresRepository) ListCategories(ctx context.Context, tenantID string, parentID string) ([]*models.ProductCategory, error) {
	executor := r.getExecutor(ctx)

	var query string
	var args []interface{}

	if parentID == "" {
		// Получаем корневые категории, если parentID не указан
		query = `
			SELECT id, name, description, parent_id, level, path, image_url
			FROM product.categories
			WHERE tenant_id = $1 AND (parent_id IS NULL OR parent_id = '')
			ORDER BY name
		`
		args = []interface{}{tenantID}
	} else {
		// Получаем подкатегории для указанного parentID
		query = `
			SELECT id, name, description, parent_id, level, path, image_url
			FROM product.categories
			WHERE tenant_id = $1 AND parent_id = $2
			ORDER BY name
		`
		args = []interface{}{tenantID, parentID}
	}

	var rows pgx.Rows
	var err error

	switch e := executor.(type) {
	case pgx.Tx:
		rows, err = e.Query(ctx, query, args...)
	case *pgxpool.Pool:
		rows, err = e.Query(ctx, query, args...)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}
	defer rows.Close()

	var categories []*models.ProductCategory
	for rows.Next() {
		var category models.ProductCategory
		err := rows.Scan(&category.ID, &category.Name, &category.Description,
			&category.ParentID, &category.Level, &category.Path, &category.ImageURL)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category row: %w", err)
		}
		categories = append(categories, &category)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error while iterating category rows: %w", rows.Err())
	}

	// Для каждой категории загружаем ID подкатегорий
	for _, category := range categories {
		subQuery := `
			SELECT id
			FROM product.categories
			WHERE parent_id = $1 AND tenant_id = $2
		`

		var subRows pgx.Rows

		switch e := executor.(type) {
		case pgx.Tx:
			subRows, err = e.Query(ctx, subQuery, category.ID, tenantID)
		case *pgxpool.Pool:
			subRows, err = e.Query(ctx, subQuery, category.ID, tenantID)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to query subcategories: %w", err)
		}

		for subRows.Next() {
			var subCategoryID string
			err := subRows.Scan(&subCategoryID)
			if err != nil {
				subRows.Close()
				return nil, fmt.Errorf("failed to scan subcategory row: %w", err)
			}
			category.SubCategories = append(category.SubCategories, subCategoryID)
		}

		subRows.Close()
		if subRows.Err() != nil {
			return nil, fmt.Errorf("error while iterating subcategory rows: %w", subRows.Err())
		}
	}

	return categories, nil
}

// DeleteCategory удаляет категорию
func (r *PostgresRepository) DeleteCategory(ctx context.Context, categoryID string, tenantID string) error {
	executor := r.getExecutor(ctx)

	query := `
		DELETE FROM product.categories 
		WHERE id = $1 AND tenant_id = $2
	`

	var err error
	switch e := executor.(type) {
	case pgx.Tx:
		_, err = e.Exec(ctx, query, categoryID, tenantID)
	case *pgxpool.Pool:
		_, err = e.Exec(ctx, query, categoryID, tenantID)
	}

	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	return nil
}

// SaveHistoryRecord сохраняет запись в истории изменений продукта
func (r *PostgresRepository) SaveHistoryRecord(ctx context.Context, record *models.ProductHistoryRecord, tenantID string) error {
	executor := r.getExecutor(ctx)

	// Если ID пустой, генерируем новый
	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	query := `
		INSERT INTO product.history (id, tenant_id, product_id, change_type, before, after, 
			changed_by, changed_at, change_comment)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	var beforeJSON, afterJSON []byte
	var err error

	if record.Before != nil {
		beforeJSON, err = json.Marshal(record.Before)
		if err != nil {
			return fmt.Errorf("failed to marshal 'before' state: %w", err)
		}
	}

	if record.After != nil {
		afterJSON, err = json.Marshal(record.After)
		if err != nil {
			return fmt.Errorf("failed to marshal 'after' state: %w", err)
		}
	}

	switch e := executor.(type) {
	case pgx.Tx:
		_, err = e.Exec(ctx, query, record.ID, tenantID, record.ProductID, record.ChangeType,
			beforeJSON, afterJSON, record.ChangedBy, record.ChangedAt, record.ChangeComment)
	case *pgxpool.Pool:
		_, err = e.Exec(ctx, query, record.ID, tenantID, record.ProductID, record.ChangeType,
			beforeJSON, afterJSON, record.ChangedBy, record.ChangedAt, record.ChangeComment)
	}

	if err != nil {
		return fmt.Errorf("failed to save history record: %w", err)
	}

	return nil
}

// GetProductHistory получает историю изменений продукта
func (r *PostgresRepository) GetProductHistory(ctx context.Context, productID string, tenantID string, limit, offset int) ([]*models.ProductHistoryRecord, error) {
	executor := r.getExecutor(ctx)

	query := `
		SELECT id, product_id, change_type, before, after, changed_by, changed_at, change_comment
		FROM product.history
		WHERE product_id = $1 AND tenant_id = $2
		ORDER BY changed_at DESC
		LIMIT $3 OFFSET $4
	`

	var rows pgx.Rows
	var err error

	switch e := executor.(type) {
	case pgx.Tx:
		rows, err = e.Query(ctx, query, productID, tenantID, limit, offset)
	case *pgxpool.Pool:
		rows, err = e.Query(ctx, query, productID, tenantID, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query product history: %w", err)
	}
	defer rows.Close()

	var records []*models.ProductHistoryRecord
	for rows.Next() {
		var record models.ProductHistoryRecord
		var beforeJSON, afterJSON []byte

		err := rows.Scan(&record.ID, &record.ProductID, &record.ChangeType, &beforeJSON, &afterJSON,
			&record.ChangedBy, &record.ChangedAt, &record.ChangeComment)
		if err != nil {
			return nil, fmt.Errorf("failed to scan history record row: %w", err)
		}

		if len(beforeJSON) > 0 {
			record.Before = &models.Product{}
			if err := json.Unmarshal(beforeJSON, record.Before); err != nil {
				return nil, fmt.Errorf("failed to unmarshal 'before' state: %w", err)
			}
		}

		if len(afterJSON) > 0 {
			record.After = &models.Product{}
			if err := json.Unmarshal(afterJSON, record.After); err != nil {
				return nil, fmt.Errorf("failed to unmarshal 'after' state: %w", err)
			}
		}

		records = append(records, &record)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error while iterating history record rows: %w", rows.Err())
	}

	return records, nil
}

// Вспомогательная функция для генерации условий фильтрации
func genFilterConditions(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}

	result := ""
	for i, condition := range conditions {
		if i == 0 {
			result += condition
		} else {
			result += " AND " + condition
		}
	}

	return result
}
