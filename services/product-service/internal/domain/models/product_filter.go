package models

// ProductFilter представляет структурированную модель для фильтрации продуктов
type ProductFilter struct {
	// Основные поля фильтрации
	ID          string   `json:"id,omitempty"`
	SupplierID  int      `json:"supplier_id,omitempty"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	CategoryID  string   `json:"category_id,omitempty"`
	CategoryIDs []string `json:"category_ids,omitempty"`

	// Фильтрация по цене
	MinPrice float64 `json:"min_price,omitempty"`
	MaxPrice float64 `json:"max_price,omitempty"`

	// Фильтрация по инвентарю
	InStock  *bool `json:"in_stock,omitempty"`
	MinStock int   `json:"min_stock,omitempty"`

	// Фильтрация по статусу
	Status   string   `json:"status,omitempty"`
	Statuses []string `json:"statuses,omitempty"`

	// Фильтрация по времени
	CreatedAfter  int64 `json:"created_after,omitempty"`  // Unix timestamp
	CreatedBefore int64 `json:"created_before,omitempty"` // Unix timestamp
	UpdatedAfter  int64 `json:"updated_after,omitempty"`  // Unix timestamp
	UpdatedBefore int64 `json:"updated_before,omitempty"` // Unix timestamp

	// Фильтрация по маркетплейсам
	MarketplaceID int `json:"marketplace_id,omitempty"`

	// Полнотекстовый поиск
	SearchQuery string `json:"search_query,omitempty"`

	// Произвольные атрибуты для фильтрации
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// ToMap преобразует ProductFilter в map для использования в запросах
func (f *ProductFilter) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if f.ID != "" {
		result["id"] = f.ID
	}

	if f.SupplierID != 0 {
		result["supplier_id"] = f.SupplierID
	}

	if f.Name != "" {
		result["name"] = f.Name
	}

	if f.Description != "" {
		result["description"] = f.Description
	}

	if f.CategoryID != "" {
		result["category_id"] = f.CategoryID
	}

	if len(f.CategoryIDs) > 0 {
		result["category_ids"] = f.CategoryIDs
	}

	if f.MinPrice > 0 {
		result["min_price"] = f.MinPrice
	}

	if f.MaxPrice > 0 {
		result["max_price"] = f.MaxPrice
	}

	if f.InStock != nil {
		result["in_stock"] = *f.InStock
	}

	if f.MinStock > 0 {
		result["min_stock"] = f.MinStock
	}

	if f.Status != "" {
		result["status"] = f.Status
	}

	if len(f.Statuses) > 0 {
		result["statuses"] = f.Statuses
	}

	if f.CreatedAfter > 0 {
		result["created_after"] = f.CreatedAfter
	}

	if f.CreatedBefore > 0 {
		result["created_before"] = f.CreatedBefore
	}

	if f.UpdatedAfter > 0 {
		result["updated_after"] = f.UpdatedAfter
	}

	if f.UpdatedBefore > 0 {
		result["updated_before"] = f.UpdatedBefore
	}

	if f.MarketplaceID > 0 {
		result["marketplace_id"] = f.MarketplaceID
	}

	if f.SearchQuery != "" {
		result["search_query"] = f.SearchQuery
	}

	if f.Attributes != nil && len(f.Attributes) > 0 {
		for key, value := range f.Attributes {
			result["attr_"+key] = value
		}
	}

	return result
}
