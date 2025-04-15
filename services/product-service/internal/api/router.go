package api

import (
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/athebyme/gomarket-platform/product-service/internal/api/handlers"
	"github.com/athebyme/gomarket-platform/product-service/internal/api/middleware"
	"github.com/athebyme/gomarket-platform/product-service/internal/domain/services"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

// SetupRouter настраивает маршрутизатор
func SetupRouter(
	productService services.ProductServiceInterface,
	logger interfaces.LoggerPort,
	corsAllowedOrigins []string,
) *chi.Mux {
	r := chi.NewRouter()

	// Глобальные middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recoverer(logger))
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.CORS(corsAllowedOrigins))
	r.Use(middleware.Tracing)

	// Маршруты health-check и метрик
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	// Маршруты API
	r.Route("/api/v1", func(r chi.Router) {
		// Middleware для аутентификации и тенанта
		r.Use(middleware.Auth)
		r.Use(middleware.Tenant)

		// Инициализация обработчиков
		productHandler := handlers.NewProductHandler(productService, logger)

		// Маршруты для продуктов
		r.Route("/products", func(r chi.Router) {
			// Получение списка продуктов
			r.Get("/", productHandler.ListProducts)

			// Создание продукта
			r.Post("/", productHandler.CreateProduct)

			// Операции с конкретным продуктом
			r.Route("/{id}", func(r chi.Router) {
				// Получение продукта по ID
				r.Get("/", productHandler.GetProduct)

				// Обновление продукта
				r.Put("/", productHandler.UpdateProduct)

				// Удаление продукта
				r.Delete("/", productHandler.DeleteProduct)

				// Синхронизация продукта с маркетплейсом
				r.Post("/sync", productHandler.SyncProductToMarketplace)
			})
		})
	})

	return r
}
