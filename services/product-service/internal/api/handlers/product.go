package handlers

import (
	"encoding/json"
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/athebyme/gomarket-platform/product-service/internal/domain/models"
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

// @title Product Service API
// @version 1.0
// @description API сервиса управления продуктами для платформы GoMarket
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

// GetProduct обрабатывает запрос на получение продукта по ID
// @Summary Получение продукта
// @Description Получает детальную информацию о продукте по его ID
// @Tags products
// @Accept json
// @Produce json
// @Param id path string true "ID продукта"
// @Param X-Tenant-ID header string true "ID тенанта"
// @Param X-Supplier-ID header string true "ID поставщика"
// @Security BearerAuth
// @Success 200 {object} response{data=models.Product} "Успешный ответ"
// @Failure 400 {object} errorResponse "Неверный запрос"
// @Failure 401 {object} errorResponse "Не авторизован"
// @Failure 403 {object} errorResponse "Запрещено"
// @Failure 404 {object} errorResponse "Продукт не найден"
// @Failure 500 {object} errorResponse "Внутренняя ошибка сервера"
// @Router /products/{id} [get]
func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
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

	supplierID := r.Header.Get("X-Supplier-ID")
	if supplierID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID поставщика не указан",
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
// @Summary Список продуктов
// @Description Получает список продуктов с поддержкой пагинации и фильтрации
// @Tags products
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "ID тенанта"
// @Param page query int false "Номер страницы" default(1) minimum(1)
// @Param page_size query int false "Размер страницы" default(20) minimum(1) maximum(100)
// @Param name query string false "Фильтр по имени продукта"
// @Param description query string false "Фильтр по описанию продукта"
// @Param supplier_id query string false "Фильтр по ID поставщика"
// @Param min_price query number false "Минимальная цена"
// @Param max_price query number false "Максимальная цена"
// @Param q query string false "Поисковый запрос"
// @Security BearerAuth
// @Success 200 {object} response{data=[]models.Product,meta=map[string]interface{}} "Успешный ответ"
// @Failure 400 {object} errorResponse "Неверный запрос"
// @Failure 401 {object} errorResponse "Не авторизован"
// @Failure 403 {object} errorResponse "Запрещено"
// @Failure 500 {object} errorResponse "Внутренняя ошибка сервера"
// @Router /products [get]

// ListProducts обрабатывает запрос на получение списка продуктов
func (h *ProductHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
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

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(r.URL.Query().Get("page_size"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filters := make(map[string]interface{})

	if name := r.URL.Query().Get("name"); name != "" {
		filters["name"] = name
	}

	if description := r.URL.Query().Get("description"); description != "" {
		filters["description"] = description
	}

	if supplierID := r.URL.Query().Get("supplier_id"); supplierID != "" {
		if id, err := strconv.Atoi(supplierID); err == nil {
			filters["supplier_id"] = id
		}
	}

	if minPrice := r.URL.Query().Get("min_price"); minPrice != "" {
		if price, err := strconv.ParseFloat(minPrice, 64); err == nil {
			filters["min_price"] = price
		}
	}

	if maxPrice := r.URL.Query().Get("max_price"); maxPrice != "" {
		if price, err := strconv.ParseFloat(maxPrice, 64); err == nil {
			filters["max_price"] = price
		}
	}

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

	pagination := utils.NewPagination(page, pageSize, "created_at", true)
	pagination.SetTotal(int64(total))

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
// @Summary Создание продукта
// @Description Создает новый продукт в системе
// @Tags products
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "ID тенанта"
// @Param X-Supplier-ID header string true "ID поставщика"
// @Param product body models.Product true "Данные продукта"
// @Security BearerAuth
// @Success 201 {object} response{data=models.Product} "Продукт создан"
// @Failure 400 {object} errorResponse "Неверный запрос"
// @Failure 401 {object} errorResponse "Не авторизован"
// @Failure 403 {object} errorResponse "Запрещено"
// @Failure 500 {object} errorResponse "Внутренняя ошибка сервера"
// @Router /products [post]
// CreateProduct обрабатывает запрос на создание продукта
func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
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
			Message: "ID поставщика не указан",
		})
		return
	}

	var product models.Product
	err := json.NewDecoder(r.Body).Decode(&product)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "Некорректный формат данных",
		})
		return
	}

	product.TenantID = tenantID
	product.SupplierID = supplierID

	var baseData map[string]interface{}
	if err := json.Unmarshal(product.BaseData, &baseData); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "validation_error",
			Code:    http.StatusBadRequest,
			Message: "Некорректный формат базовых данных продукта",
		})
		return
	}

	if name, ok := baseData["name"].(string); !ok || name == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "validation_error",
			Code:    http.StatusBadRequest,
			Message: "Название продукта не может быть пустым",
		})
		return
	}

	if price, ok := baseData["price"].(float64); !ok || price <= 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "validation_error",
			Code:    http.StatusBadRequest,
			Message: "Цена продукта должна быть больше нуля",
		})
		return
	}

	createdProduct, err := h.productService.CreateProduct(r.Context(), &product)
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
// @Summary Обновление продукта
// @Description Обновляет существующий продукт по его ID
// @Tags products
// @Accept json
// @Produce json
// @Param id path string true "ID продукта"
// @Param X-Tenant-ID header string true "ID тенанта"
// @Param product body models.Product true "Данные продукта"
// @Security BearerAuth
// @Success 200 {object} response{data=models.Product} "Продукт обновлен"
// @Failure 400 {object} errorResponse "Неверный запрос"
// @Failure 401 {object} errorResponse "Не авторизован"
// @Failure 403 {object} errorResponse "Запрещено"
// @Failure 404 {object} errorResponse "Продукт не найден"
// @Failure 500 {object} errorResponse "Внутренняя ошибка сервера"
// @Router /products/{id} [put]
func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
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

	var product models.Product
	err := json.NewDecoder(r.Body).Decode(&product)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "Некорректный формат данных",
		})
		return
	}

	product.ID = productID
	product.TenantID = tenantID

	var baseData map[string]interface{}
	if err := json.Unmarshal(product.BaseData, &baseData); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "validation_error",
			Code:    http.StatusBadRequest,
			Message: "Некорректный формат базовых данных продукта",
		})
		return
	}

	if name, ok := baseData["name"].(string); !ok || name == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "validation_error",
			Code:    http.StatusBadRequest,
			Message: "Название продукта не может быть пустым",
		})
		return
	}

	if price, ok := baseData["price"].(float64); !ok || price <= 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "validation_error",
			Code:    http.StatusBadRequest,
			Message: "Цена продукта должна быть больше нуля",
		})
		return
	}

	updatedProduct, err := h.productService.UpdateProduct(r.Context(), &product)
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

	render.Status(r, http.StatusOK)
	render.JSON(w, r, response{
		Success: true,
		Data:    updatedProduct,
	})
}

// DeleteProduct обрабатывает запрос на удаление продукта
// @Summary Удаление продукта
// @Description Удаляет продукт по его ID
// @Tags products
// @Accept json
// @Produce json
// @Param id path string true "ID продукта"
// @Param X-Tenant-ID header string true "ID тенанта"
// @Param X-Supplier-ID header string true "ID поставщика"
// @Security BearerAuth
// @Success 200 {object} response{data=map[string]interface{}} "Продукт удален"
// @Failure 400 {object} errorResponse "Неверный запрос"
// @Failure 401 {object} errorResponse "Не авторизован"
// @Failure 403 {object} errorResponse "Запрещено"
// @Failure 404 {object} errorResponse "Продукт не найден"
// @Failure 500 {object} errorResponse "Внутренняя ошибка сервера"
// @Router /products/{id} [delete]
func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
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

	supplierID := r.Header.Get("X-Supplier-ID")
	if supplierID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Error:   "bad_request",
			Code:    http.StatusBadRequest,
			Message: "ID поставщика не указан",
		})
		return
	}

	err := h.productService.DeleteProduct(r.Context(), productID, supplierID, tenantID)
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
// @Summary Синхронизация с маркетплейсом
// @Description Синхронизирует продукт с выбранным маркетплейсом
// @Tags products
// @Accept json
// @Produce json
// @Param id path string true "ID продукта"
// @Param X-Tenant-ID header string true "ID тенанта"
// @Param marketplace_id query int true "ID маркетплейса"
// @Security BearerAuth
// @Success 200 {object} response{data=map[string]interface{}} "Синхронизация запущена"
// @Failure 400 {object} errorResponse "Неверный запрос"
// @Failure 401 {object} errorResponse "Не авторизован"
// @Failure 403 {object} errorResponse "Запрещено"
// @Failure 404 {object} errorResponse "Продукт не найден"
// @Failure 500 {object} errorResponse "Внутренняя ошибка сервера"
// @Router /products/{id}/sync [post]
func (h *ProductHandler) SyncProductToMarketplace(w http.ResponseWriter, r *http.Request) {
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
