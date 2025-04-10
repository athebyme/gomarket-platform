package models

import (
	"encoding/json"
	"time"
)

// Product представляет модель товара для продажи на маркетплейсе
type Product struct {
	ID         string          `json:"id"`
	SupplierID string          `json:"supplier_id"`
	TenantID   string          `json:"tenant_id"`
	BaseData   json.RawMessage `db:"base_data" json:"base_data"`
	// Metadata хранит в себе информацию, необходимую для системы
	Metadata  json.RawMessage `db:"metadata" json:"metadata,omitempty"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}

// ProductInventory представляет собой модель описания остатков товара
type ProductInventory struct {
	ProductID  string    `json:"product_id"`
	SupplierID int       `json:"supplier_id"`
	Quantity   int       `json:"quantity"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProductPrice представляет собой модель цен для товаров
type ProductPrice struct {
	ProductID    string    `json:"product_id"`
	SupplierID   int       `json:"supplier_id"`
	BasePrice    float64   `json:"base_price"`
	SpecialPrice float64   `json:"special_price,omitempty"`
	Currency     string    `json:"currency"`
	StartDate    time.Time `json:"start_date,omitempty"`
	EndDate      time.Time `json:"end_date,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ProductMedia представляет собой модель медиа-файлов товара
type ProductMedia struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	Type      string    `json:"type"` // "image", "video", etc.
	URL       string    `json:"url"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// ---------------------------- KAFKA MODELS ----------------------------

// ProductHistoryRecord представляет собой записи в истории изменений продукта для Kafka
type ProductHistoryRecord struct {
	ID            string   `json:"id"`
	ProductID     string   `json:"product_id"`
	ChangeType    string   `json:"change_type"` // "create", "update", "delete"
	Before        *Product `json:"before,omitempty"`
	After         *Product `json:"after,omitempty"`
	ChangedBy     string   `json:"changed_by,omitempty"`
	ChangedAt     int64    `json:"changed_at"`
	ChangeComment string   `json:"change_comment,omitempty"`
}
