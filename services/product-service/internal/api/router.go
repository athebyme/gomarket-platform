package api

import (
	"github.com/athebyme/gomarket-platform/pkg/auth"
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/athebyme/gomarket-platform/product-service/internal/api/handlers"
	"github.com/athebyme/gomarket-platform/product-service/internal/api/middleware"
	"github.com/athebyme/gomarket-platform/product-service/internal/domain/services"
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
	keycloakClient *auth.KeycloakClient,
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
		r.Use(middleware.KeycloakAuth(keycloakClient, logger))
		r.Use(middleware.Tenant)
		r.Use(middleware.Supplier)

		productHandler := handlers.NewProductHandler(productService, logger)

		// Маршруты для продуктов
		r.Route("/products", func(r chi.Router) {
			// Получение списка продуктов
			r.With(middleware.RequireProductPermission(keycloakClient, "read")).Get("/", productHandler.ListProducts)

			// Создание продукта
			r.With(middleware.RequireProductPermission(keycloakClient, "create")).Post("/", productHandler.CreateProduct)

			// Операции с конкретным продуктом
			r.Route("/{id}", func(r chi.Router) {
				// Получение продукта по ID
				r.With(middleware.RequireProductPermission(keycloakClient, "read")).Get("/", productHandler.GetProduct)

				// Обновление продукта
				r.With(middleware.RequireProductPermission(keycloakClient, "update")).Put("/", productHandler.UpdateProduct)

				// Удаление продукта
				r.With(middleware.RequireProductPermission(keycloakClient, "delete")).Delete("/", productHandler.DeleteProduct)

				// Синхронизация продукта с маркетплейсом
				r.With(middleware.RequireProductPermission(keycloakClient, "sync")).Post("/sync", productHandler.SyncProductToMarketplace)
			})
		})
	})

	return r
}
