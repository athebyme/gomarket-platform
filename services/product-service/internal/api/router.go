package api

import (
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/athebyme/gomarket-platform/product-service/internal/api/handlers"
	"github.com/athebyme/gomarket-platform/product-service/internal/api/middleware"
	"github.com/athebyme/gomarket-platform/product-service/internal/domain/services"
	"github.com/athebyme/gomarket-platform/product-service/internal/security"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
	"net/http"
	"time"
)

// SetupRouter настраивает маршрутизатор
func SetupRouter(
	productService services.ProductServiceInterface,
	logger interfaces.LoggerPort,
	corsAllowedOrigins []string,
	jwtManager *security.JWTManager,
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
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.RateLimiter(1000, time.Minute))

	r.Method(http.MethodGet, "/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	r.Method(http.MethodHead, "/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.JWTAuth(jwtManager, logger))
		r.Use(middleware.CSRF) // Защита от CSRF

		productHandler := handlers.NewProductHandler(productService, logger)

		// Маршруты для продуктов
		r.Route("/products", func(r chi.Router) {
			// Получение списка продуктов
			r.With(middleware.HasPermission("products:read")).Get("/", productHandler.ListProducts)

			// Создание продукта
			r.With(middleware.HasPermission("products:create")).Post("/", productHandler.CreateProduct)

			// Операции с конкретным продуктом
			r.Route("/{id}", func(r chi.Router) {
				// Получение продукта по ID
				r.With(middleware.HasPermission("products:read")).Get("/", productHandler.GetProduct)

				// Обновление продукта
				r.With(middleware.HasPermission("products:update")).Put("/", productHandler.UpdateProduct)

				// Удаление продукта
				r.With(middleware.HasPermission("products:delete")).Delete("/", productHandler.DeleteProduct)

				// Синхронизация продукта с маркетплейсом
				r.With(middleware.HasPermission("products:sync")).Post("/sync", productHandler.SyncProductToMarketplace)
			})
		})
	})

	return r
}
