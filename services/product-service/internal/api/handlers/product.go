package handlers

import (
	"encoding/json"
	"github.com/athebyme/gomarket-platform/pkg/dto"
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/athebyme/gomarket-platform/product-service/internal/domain/services"
	"github.com/athebyme/gomarket-platform/product-service/internal/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"net/http"
	"strconv"
)

// ProductHandler обработчик запросов для продуктов
type ProductHandler struct {
	productService services.ProductServiceInterface
	logger         interfaces.LoggerPort
}

// NewProductHandler создает новый обработчик продуктов
func NewProductHandler(productService services.ProductServiceInterface, logger interfaces.LoggerPort) *ProductHandler {
	return &ProductHandler{
		productService: productService,
		logger:         logger,
	}
}

// errorResponse представляет структуру ответа с ошибкой
type errorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// response представляет структуру успешного ответа
type response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

// GetProduct обрабатывает запрос на получение продукта по ID
func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	// Получаем ID продукта из URL
	productID := chi.URLParam(r, "id")
	if productID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID продукта не указан",
		})
		return
	}

	// Получаем ID тенанта из контекста
	tenantID, ok := r.Context().Value("tenant_id").(string)
	if !ok || tenantID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID тенанта не указан",
		})
		return
	}

	supplierID, ok := r.Context().Value("supplier_id").(string)
	if !ok || supplierID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID тенанта не указан",
		})
		return
	}

	product, err := h.productService.GetProduct(r.Context(), productID, supplierID, tenantID)
	if err != nil {
		h.logger.ErrorWithContext(r.Context(), "Ошибка получения продукта",
			interfaces.LogField{Key: "error", Value: err.Error()})
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, errorResponse{
			Error:   "internal_error",
			Code:    http.StatusInternalServerError,
			Message: "Ошибка получения продукта",
		})
		return
	}

	if product == nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, errorResponse{
			Error:   "not_found",
			Code:    http.StatusNotFound,
			Message: "Продукт не найден",
		})
		return
	}

	// Возвращаем продукт
	render.Status(r, http.StatusOK)
	render.JSON(w, r, response{
		Success: true,
		Data:    product,
	})
}

// ListProducts обрабатывает запрос на получение списка продуктов
func (h *ProductHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	// Получаем ID тенанта из контекста
	tenantID, ok := r.Context().Value("tenant_id").(string)
	if !ok || tenantID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID тенанта не указан",
		})
		return
	}

	// Получаем параметры пагинации
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(r.URL.Query().Get("page_size"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Получаем параметры фильтрации
	filters := make(map[string]interface{})

	// Фильтр по имени
	if name := r.URL.Query().Get("name"); name != "" {
		filters["name"] = name
	}

	// Фильтр по описанию
	if description := r.URL.Query().Get("description"); description != "" {
		filters["description"] = description
	}

	// Фильтр по ID поставщика
	if supplierID := r.URL.Query().Get("supplier_id"); supplierID != "" {
		if id, err := strconv.Atoi(supplierID); err == nil {
			filters["supplier_id"] = id
		}
	}

	// Фильтр по минимальной цене
	if minPrice := r.URL.Query().Get("min_price"); minPrice != "" {
		if price, err := strconv.ParseFloat(minPrice, 64); err == nil {
			filters["min_price"] = price
		}
	}

	// Фильтр по максимальной цене
	if maxPrice := r.URL.Query().Get("max_price"); maxPrice != "" {
		if price, err := strconv.ParseFloat(maxPrice, 64); err == nil {
			filters["max_price"] = price
		}
	}

	// Фильтр по поисковому запросу
	if query := r.URL.Query().Get("q"); query != "" {
		filters["search_query"] = query
	}

	products, total, err := h.productService.ListProducts(r.Context(), tenantID, filters, page, pageSize)
	if err != nil {
		h.logger.ErrorWithContext(r.Context(), "Ошибка получения списка продуктов",
			interfaces.LogField{Key: "error", Value: err.Error()})
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, errorResponse{
			Error:   "internal_error",
			Code:    http.StatusInternalServerError,
			Message: "Ошибка получения списка продуктов",
		})
		return
	}

	// Создаем пагинацию
	pagination := utils.NewPagination(page, pageSize, "created_at", true)
	pagination.SetTotal(int64(total))

	// Возвращаем продукты
	render.Status(r, http.StatusOK)
	render.JSON(w, r, response{
		Success: true,
		Data:    products,
		Meta: map[string]interface{}{
			"pagination": pagination,
		},
	})
}

// CreateProduct обрабатывает запрос на создание продукта
func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	// Получаем ID тенанта из контекста
	tenantID, ok := r.Context().Value("tenant_id").(string)
	if !ok || tenantID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID тенанта не указан",
		})
		return
	}

	// Декодируем тело запроса
	var productDTO dto.ProductDTO
	err := json.NewDecoder(r.Body).Decode(&productDTO)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "Некорректный формат данных",
		})
		return
	}

	// Валидируем продукт
	if productDTO.Name == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "validation_error",
			Code:    http.StatusBadRequest,
			Message: "Название продукта не может быть пустым",
		})
		return
	}

	if productDTO.Price <= 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "validation_error",
			Code:    http.StatusBadRequest,
			Message: "Цена продукта должна быть больше нуля",
		})
		return
	}

	// Создаем продукт
	createdProduct, err := h.productService.CreateProduct(r.Context(), &productDTO, tenantID)
	if err != nil {
		h.logger.ErrorWithContext(r.Context(), "Ошибка создания продукта",
			interfaces.LogField{Key: "error", Value: err.Error()})
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, errorResponse{
			Error:   "internal_error",
			Code:    http.StatusInternalServerError,
			Message: "Ошибка создания продукта",
		})
		return
	}

	// Возвращаем созданный продукт
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, response{
		Success: true,
		Data:    createdProduct,
	})
}

// UpdateProduct обрабатывает запрос на обновление продукта
func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	// Получаем ID продукта из URL
	productID := chi.URLParam(r, "id")
	if productID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID продукта не указан",
		})
		return
	}

	// Получаем ID тенанта из контекста
	tenantID, ok := r.Context().Value("tenant_id").(string)
	if !ok || tenantID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID тенанта не указан",
		})
		return
	}

	// Декодируем тело запроса
	var productDTO dto.ProductDTO
	err := json.NewDecoder(r.Body).Decode(&productDTO)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "Некорректный формат данных",
		})
		return
	}

	// Устанавливаем ID продукта
	productDTO.ID = productID

	// Валидируем продукт
	if productDTO.Name == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "validation_error",
			Code:    http.StatusBadRequest,
			Message: "Название продукта не может быть пустым",
		})
		return
	}

	if productDTO.Price <= 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "validation_error",
			Code:    http.StatusBadRequest,
			Message: "Цена продукта должна быть больше нуля",
		})
		return
	}

	// Обновляем продукт
	updatedProduct, err := h.productService.UpdateProduct(r.Context(), &productDTO, tenantID)
	if err != nil {
		h.logger.ErrorWithContext(r.Context(), "Ошибка обновления продукта",
			interfaces.LogField{Key: "error", Value: err.Error()})
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, errorResponse{
			Error:   "internal_error",
			Code:    http.StatusInternalServerError,
			Message: "Ошибка обновления продукта",
		})
		return
	}

	// Возвращаем обновленный продукт
	render.Status(r, http.StatusOK)
	render.JSON(w, r, response{
		Success: true,
		Data:    updatedProduct,
	})
}

// DeleteProduct обрабатывает запрос на удаление продукта
func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	// Получаем ID продукта из URL
	productID := chi.URLParam(r, "id")
	if productID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID продукта не указан",
		})
		return
	}

	// Получаем ID тенанта из контекста
	tenantID, ok := r.Context().Value("tenant_id").(string)
	if !ok || tenantID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID тенанта не указан",
		})
		return
	}

	// Удаляем продукт
	err := h.productService.DeleteProduct(r.Context(), productID, tenantID)
	if err != nil {
		h.logger.ErrorWithContext(r.Context(), "Ошибка удаления продукта",
			interfaces.LogField{Key: "error", Value: err.Error()})
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, errorResponse{
			Error:   "internal_error",
			Code:    http.StatusInternalServerError,
			Message: "Ошибка удаления продукта",
		})
		return
	}

	// Возвращаем успешный ответ
	render.Status(r, http.StatusOK)
	render.JSON(w, r, response{
		Success: true,
		Data: map[string]interface{}{
			"id":      productID,
			"deleted": true,
		},
	})
}

// SyncProductToMarketplace синхронизирует продукт с маркетплейсом
func (h *ProductHandler) SyncProductToMarketplace(w http.ResponseWriter, r *http.Request) {
	// Получаем ID продукта из URL
	productID := chi.URLParam(r, "id")
	if productID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID продукта не указан",
		})
		return
	}

	// Получаем ID тенанта из контекста
	tenantID, ok := r.Context().Value("tenant_id").(string)
	if !ok || tenantID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID тенанта не указан",
		})
		return
	}

	// Получаем ID маркетплейса из параметров запроса
	marketplaceIDStr := r.URL.Query().Get("marketplace_id")
	if marketplaceIDStr == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID маркетплейса не указан",
		})
		return
	}

	marketplaceID, err := strconv.Atoi(marketplaceIDStr)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "Некорректный ID маркетплейса",
		})
		return
	}

	// Синхронизируем продукт с маркетплейсом
	err = h.productService.SyncProductToMarketplace(r.Context(), productID, marketplaceID, tenantID)
	if err != nil {
		h.logger.ErrorWithContext(r.Context(), "Ошибка синхронизации продукта с маркетплейсом",
			interfaces.LogField{Key: "error", Value: err.Error()})
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, errorResponse{
			Error:   "internal_error",
			Code:    http.StatusInternalServerError,
			Message: "Ошибка синхронизации продукта с маркетплейсом",
		})
		return
	}

	// Возвращаем успешный ответ
	render.Status(r, http.StatusOK)
	render.JSON(w, r, response{
		Success: true,
		Data: map[string]interface{}{
			"product_id":     productID,
			"marketplace_id": marketplaceID,
			"synced":         true,
		},
	})
}
