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
	"github.com/athebyme/gomarket-platform/product-service/internal/utils"
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

	cacheOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_operations_total",
		Help: "Количество операций с кэшем",
	}, []string{"operation", "status"})
)

func main() {
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("Ошибка загрузки конфигурации: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	postgresCon, err := utils.GenerateConnectionString(
		cfg.Postgres.Host,
		cfg.Postgres.User,
		cfg.Postgres.Password,
		cfg.Postgres.DBName,
		cfg.Postgres.SSLMode,
		cfg.Postgres.Port,
		cfg.Postgres.PoolSize,
		cfg.Postgres.Timeout,
	)
	if err != nil {
		fmt.Printf("Ошибка инициализации строки подключения базы: %v\n", err)
		os.Exit(1)
	}

	db, err := postgres.NewPostgresStorage(
		ctx,
		postgresCon,
	)
	if err != nil {
		log.Fatal("Ошибка инициализации хранилища", interfaces.LogField{Key: "error", Value: err.Error()})
	}
	defer db.Close()
	log.Info("Хранилище инициализировано")

	testCtx, testCancel := context.WithTimeout(ctx, 5*time.Second)
	defer testCancel()

	if err := checkPostgresConnection(testCtx, db); err != nil {
		log.Fatal("Ошибка подключения к PostgreSQL",
			interfaces.LogField{Key: "error", Value: err.Error()})
	}
	log.Info("Соединение с PostgreSQL проверено")

	cacheClient, err := cache.NewRedisCache(
		ctx,
		cfg.Redis.Host,
		cfg.Redis.Port,
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	if err != nil {
		log.Fatal("Ошибка инициализации кэша", interfaces.LogField{Key: "error", Value: err.Error()})
	}
	defer cacheClient.Close()
	log.Info("Кэш инициализирован")

	if err := checkRedisConnection(testCtx, cacheClient); err != nil {
		log.Fatal("Ошибка подключения к Redis",
			interfaces.LogField{Key: "error", Value: err.Error()})
	}
	log.Info("Соединение с Redis проверено")

	messagingClient, err := messaging.NewKafkaMessaging(
		cfg.Kafka.Brokers,
		cfg.Kafka.GroupID,
		cfg.Kafka.DeadLetterTopic,
		log,
	)
	if err != nil {
		log.Fatal("Ошибка инициализации системы обмена сообщениями", interfaces.LogField{Key: "error", Value: err.Error()})
	}
	defer messagingClient.Close()
	log.Info("Система обмена сообщениями инициализирована")

	productService := services.NewProductService(db, cacheClient, messagingClient, log)
	log.Info("Сервис продуктов инициализирован")

	router := api.SetupRouter(productService, log, cfg.Security.CORSAllowOrigins)
	log.Info("Маршрутизатор настроен")

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("Сервер запущен", interfaces.LogField{Key: "address", Value: server.Addr})
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Ошибка запуска сервера", interfaces.LogField{Key: "error", Value: err.Error()})
		}
	}()

	go func() {
		<-quit
		log.Info("Получен сигнал завершения, выполняется graceful shutdown...")

		ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatal("Ошибка при graceful shutdown", interfaces.LogField{Key: "error", Value: err.Error()})
		}

		log.Info("HTTP сервер остановлен")

		log.Info("Закрытие соединений с зависимостями...")

		if err := messagingClient.Close(); err != nil {
			log.Error("Ошибка при закрытии Kafka",
				interfaces.LogField{Key: "error", Value: err.Error()})
		}

		if err := cacheClient.Close(); err != nil {
			log.Error("Ошибка при закрытии Redis",
				interfaces.LogField{Key: "error", Value: err.Error()})
		}

		if err := db.Close(); err != nil {
			log.Error("Ошибка при закрытии БД",
				interfaces.LogField{Key: "error", Value: err.Error()})
		}

		close(done)
	}()

	// Ожидаем завершения работы
	<-done
	log.Info("Сервер корректно завершил работу")
}

// Проверка соединения с PostgreSQL
func checkPostgresConnection(ctx context.Context, db interfaces.StoragePort) error {
	_, err := db.BeginTx(ctx)
	return err
}

// Проверка соединения с Redis
func checkRedisConnection(ctx context.Context, cacheClient interfaces.CachePort) error {
	testKey := "test:connection"
	testValue := []byte("test-value")

	// Попытка записи в Redis
	if err := cacheClient.Set(ctx, testKey, testValue, 10*time.Second); err != nil {
		return fmt.Errorf("ошибка записи в Redis: %w", err)
	}

	// Попытка чтения из Redis
	value, err := cacheClient.Get(ctx, testKey)
	if err != nil {
		return fmt.Errorf("ошибка чтения из Redis: %w", err)
	}

	// Проверка значения
	if string(value) != string(testValue) {
		return fmt.Errorf("некорректное значение из Redis: получено %s, ожидалось %s",
			string(value), string(testValue))
	}

	// Удаление тестового ключа
	if err := cacheClient.Delete(ctx, testKey); err != nil {
		return fmt.Errorf("ошибка удаления из Redis: %w", err)
	}

	return nil
}
