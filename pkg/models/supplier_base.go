package models

import "time"

// Supplier представляет поставщика в системе.
// В базе данных этот объект хранится в таблице core.suppliers.
type Supplier struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
