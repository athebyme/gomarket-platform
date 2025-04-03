package models

// ProductCategory представляет категорию продуктов
type ProductCategory struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	ParentID      string   `json:"parent_id,omitempty"`
	Level         int      `json:"level"`
	Path          string   `json:"path"`
	ImageURL      string   `json:"image_url,omitempty"`
	SubCategories []string `json:"sub_categories,omitempty"`
}
