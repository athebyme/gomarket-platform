package main

import (
	"context"
	"fmt"
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/athebyme/gomarket-platform/product-service/config"
	"github.com/athebyme/gomarket-platform/product-service/internal/adapters/cache"
	"github.com/athebyme/gomarket-platform/product-service/internal/adapters/logger"
	"github.com/athebyme/gomarket-platform/product-service/internal/adapters/messaging"
	"github.com/athebyme/gomarket-platform/product-service/internal/adapters/storage"
	"github.com/athebyme/gomarket-platform/product-service/internal/api"
	"github.com/athebyme/gomarket-platform/product-service/internal/domain/services"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// метрики для Prometheus
var (
	httpDurations = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_durations_seconds",
		Help:    "Длительность HTTP запросов",
		Buckets: prometheus.DefBuckets,
	}, []string{"path", "method", "status"})

	requestsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Общее количество HTTP запросов",
	}, []string{"path", "method", "status"})

	activeRequests = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "http_active_requests",
		Help: "Количество активных HTTP запросов",
	})
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("Ошибка загрузки конфигурации: %v\n", err)
		os.Exit(1)
	}

	// Создаем корневой контекст
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализируем логгер
	log, err := logger.NewZapLogger(cfg.LogLevel, cfg.ENV == "production")
	if err != nil {
		fmt.Printf("Ошибка инициализации логгера: %v\n", err)
		os.Exit(1)
	}
	log.Info("Инициализация сервиса",
		interfaces.LogField{Key: "app_name", Value: cfg.AppName},
		interfaces.LogField{Key: "version", Value: cfg.Version},
		interfaces.LogField{Key: "env", Value: cfg.ENV},
	)

	// Инициализируем хранилище
	db, err := storage.NewPostgresStorage(
		ctx,
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Postgres.User,
		cfg.Postgres.Password,
		cfg.Postgres.DBName,
		cfg.Postgres.SSLMode,
		cfg.Postgres.Timeout,
		cfg.Postgres.PoolSize,
	)
	if err != nil {
		log.Fatal("Ошибка инициализации хранилища", interfaces.LogField{Key: "error", Value: err.Error()})
	}
	defer db.Close()
	log.Info("Хранилище инициализировано")

	// Инициализируем кэш
	cacheClient, err := cache.NewRedisCache(
		ctx,
		cfg.Redis.Host,
		cfg.Redis.Port,
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.Redis.PoolSize,
		cfg.Redis.MinIdleConns,
	)
	if err != nil {
		log.Fatal("Ошибка инициализации кэша", interfaces.LogField{Key: "error", Value: err.Error()})
	}
	defer cacheClient.Close()
	log.Info("Кэш инициализирован")

	// Инициализируем систему обмена сообщениями
	messagingClient, err := messaging.NewKafkaMessaging(cfg.Kafka.Brokers, cfg.Kafka.GroupID)
	if err != nil {
		log.Fatal("Ошибка инициализации системы обмена сообщениями", interfaces.LogField{Key: "error", Value: err.Error()})
	}
	defer messagingClient.Close()
	log.Info("Система обмена сообщениями инициализирована")

	// Инициализируем сервис продуктов
	productService, err := services.NewProductService(db, cacheClient, messagingClient, log)
	if err != nil {
		log.Fatal("Ошибка инициализации сервиса продуктов", interfaces.LogField{Key: "error", Value: err.Error()})
	}
	log.Info("Сервис продуктов инициализирован")

	// Настраиваем маршрутизатор
	router := api.SetupRouter(productService, log, cfg.Security.CORSAllowOrigins)
	log.Info("Маршрутизатор настроен")

	// Создаем HTTP сервер
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// Обрабатываем сигналы для graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем сервер
	go func() {
		log.Info("Сервер запущен", interfaces.LogField{Key: "address", Value: server.Addr})
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Ошибка запуска сервера", interfaces.LogField{Key: "error", Value: err.Error()})
		}
	}()

	//  сигнал завершения
	go func() {
		<-quit
		log.Info("Получен сигнал завершения, выполняется graceful shutdown...")

		//  контекст с таймаутом для graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		// Останавливаем сервер
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal("Ошибка при graceful shutdown", interfaces.LogField{Key: "error", Value: err.Error()})
		}

		// Завершаем работу
		close(done)
	}()

	// завершение работы
	<-done
	log.Info("Сервер корректно завершил работу")
}
