package models

// MarketplaceProduct представляет продукт, существующий в конкретном маркетплейсе
type MarketplaceProduct struct {
	ID            string `json:"id"`                       // Уникальный идентификатор в нашей системе
	MarketplaceID int    `json:"marketplace_id"`           // ID маркетплейса
	CoreProductID string `json:"core_product_id"`          // ID товара в основной системе
	ExternalID    string `json:"external_id"`              // ID в системе маркетплейса
	Status        string `json:"status"`                   // "active", "pending", "rejected" и т.д.
	StatusMessage string `json:"status_message,omitempty"` // Сообщение о статусе (опционально)
}
