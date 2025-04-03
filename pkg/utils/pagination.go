package utils

// Pagination представляет расширенную модель для пагинации
type Pagination struct {
	Page       int    `json:"page"`        // Номер страницы (начиная с 1)
	PageSize   int    `json:"page_size"`   // Размер страницы
	TotalItems int64  `json:"total_items"` // Общее количество элементов
	TotalPages int    `json:"total_pages"` // Общее количество страниц
	SortBy     string `json:"sort_by"`     // Поле для сортировки
	SortDesc   bool   `json:"sort_desc"`   // Сортировка по убыванию
	HasNext    bool   `json:"has_next"`    // Есть ли следующая страница
	HasPrev    bool   `json:"has_prev"`    // Есть ли предыдущая страница
}

// NewPagination создает новый экземпляр Pagination с заданными параметрами
func NewPagination(page, pageSize int, sortBy string, sortDesc bool) *Pagination {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 10
	}

	return &Pagination{
		Page:       page,
		PageSize:   pageSize,
		SortBy:     sortBy,
		SortDesc:   sortDesc,
		TotalItems: 0,
		TotalPages: 0,
		HasNext:    false,
		HasPrev:    false,
	}
}

// SetTotal устанавливает общее количество элементов и пересчитывает зависимые поля
func (p *Pagination) SetTotal(totalItems int64) {
	p.TotalItems = totalItems
	p.TotalPages = int((totalItems + int64(p.PageSize) - 1) / int64(p.PageSize))
	p.HasNext = p.Page < p.TotalPages
	p.HasPrev = p.Page > 1
}

// GetOffset возвращает смещение для SQL запроса
func (p *Pagination) GetOffset() int {
	return (p.Page - 1) * p.PageSize
}

// GetLimit возвращает лимит для SQL запроса
func (p *Pagination) GetLimit() int {
	return p.PageSize
}

// GetSortOrder возвращает строку порядка сортировки для SQL запроса
func (p *Pagination) GetSortOrder() string {
	if p.SortBy == "" {
		return "created_at DESC"
	}

	direction := "ASC"
	if p.SortDesc {
		direction = "DESC"
	}

	return p.SortBy + " " + direction
}

// PagedResult представляет результат запроса с пагинацией
type PagedResult struct {
	Items      interface{} `json:"items"`      // Элементы текущей страницы
	Pagination *Pagination `json:"pagination"` // Информация о пагинации
}

// NewPagedResult создает новый результат с пагинацией
func NewPagedResult(items interface{}, pagination *Pagination) *PagedResult {
	return &PagedResult{
		Items:      items,
		Pagination: pagination,
	}
}
