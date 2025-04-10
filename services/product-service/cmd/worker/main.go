package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/athebyme/gomarket-platform/product-service/config"
	"github.com/athebyme/gomarket-platform/product-service/internal/adapters/cache"
	"github.com/athebyme/gomarket-platform/product-service/internal/adapters/logger"
	"github.com/athebyme/gomarket-platform/product-service/internal/adapters/messaging"
	"github.com/athebyme/gomarket-platform/product-service/internal/adapters/storage"
	"github.com/athebyme/gomarket-platform/product-service/internal/domain/services"
	"github.com/athebyme/gomarket-platform/product-service/internal/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// метрики для Prometheus
var (
	messagesProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "worker_messages_processed_total",
		Help: "Общее количество обработанных сообщений",
	}, []string{"topic", "status"})

	messageProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "worker_message_processing_duration_seconds",
		Help:    "Длительность обработки сообщений",
		Buckets: prometheus.DefBuckets,
	}, []string{"topic"})

	activeWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "worker_active_goroutines",
		Help: "Количество активных горутин-обработчиков",
	})
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
	log.Info("Инициализация воркера",
		interfaces.LogField{Key: "app_name", Value: cfg.AppName + "-worker"},
		interfaces.LogField{Key: "version", Value: cfg.Version},
		interfaces.LogField{Key: "env", Value: cfg.ENV},
	)

	connectionStr, err := utils.GenerateConnectionString(
		cfg.Postgres.Host,
		cfg.Postgres.User,
		cfg.Postgres.Password,
		cfg.Postgres.DBName,
		cfg.Postgres.SSLMode,
		cfg.Postgres.PoolSize,
		cfg.Postgres.Port,
		cfg.Postgres.Timeout,
	)

	repo, err := postgres.NewPostgresRepository(
		ctx,
		connectionStr,
	)
	if err != nil {
		log.Fatal("Ошибка инициализации хранилища", interfaces.LogField{Key: "error", Value: err.Error()})
	}
	defer repo.Close()

	log.Info("Хранилище инициализировано")

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

	messagingClient, err := messaging.NewKafkaMessaging(cfg.Kafka.Brokers, cfg.Kafka.GroupID)
	if err != nil {
		log.Fatal("Ошибка инициализации системы обмена сообщениями", interfaces.LogField{Key: "error", Value: err.Error()})
	}
	defer messagingClient.Close()
	log.Info("Система обмена сообщениями инициализирована")

	productService := services.NewProductService(repo, cacheClient, messagingClient, log)
	if err != nil {
		log.Fatal("Ошибка инициализации сервиса продуктов", interfaces.LogField{Key: "error", Value: err.Error()})
	}
	log.Info("Сервис продуктов инициализирован")

	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	subscribeToProductCommands(ctx, messagingClient, productService, log, &wg)
	subscribeToProductEvents(ctx, messagingClient, productService, log, &wg)

	go func() {
		<-quit
		log.Info("Получен сигнал завершения, выполняется graceful shutdown...")

		cancel()

		wg.Wait()

		close(done)
	}()

	<-done
	log.Info("Воркер корректно завершил работу")
}

// subscribeToProductEvents подписывается на события продуктов
func subscribeToProductEvents(
	ctx context.Context,
	messagingClient interfaces.MessagingPort,
	productService services.ProductServiceInterface,
	log interfaces.LoggerPort,
	wg *sync.WaitGroup,
) {
	eventHandler := func(ctx context.Context, msg *interfaces.Message) error {
		startTime := time.Now()
		activeWorkers.Inc()
		defer activeWorkers.Dec()

		log.InfoWithContext(ctx, "Получено событие продукта",
			interfaces.LogField{Key: "message_id", Value: msg.ID},
			interfaces.LogField{Key: "topic", Value: msg.Topic},
		)

		var event struct {
			EventType  string                 `json:"event_type"`
			TenantID   string                 `json:"tenant_id"`
			SupplierID string                 `json:"supplier_id"`
			Payload    map[string]interface{} `json:"payload"`
		}

		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.ErrorWithContext(ctx, "Ошибка декодирования события",
				interfaces.LogField{Key: "error", Value: err.Error()},
				interfaces.LogField{Key: "message_id", Value: msg.ID},
			)
			messagesProcessed.WithLabelValues(msg.Topic, "error").Inc()
			return err
		}

		evtCtx := context.WithValue(ctx, "tenant_id", event.TenantID)

		switch event.EventType {
		case "product_price_updated":
			productID, _ := event.Payload["product_id"].(string)
			price, _ := event.Payload["price"].(float64)

			log.InfoWithContext(evtCtx, "Обработка события обновления цены",
				interfaces.LogField{Key: "product_id", Value: productID},
				interfaces.LogField{Key: "price", Value: price},
			)

			cacheKey := fmt.Sprintf("product:%s", productID)
			productService.InvalidateCache(evtCtx, cacheKey, event.TenantID)

		case "product_inventory_updated":
			productID, _ := event.Payload["product_id"].(string)
			quantity, _ := event.Payload["quantity"].(float64)

			log.InfoWithContext(evtCtx, "Обработка события обновления инвентаря",
				interfaces.LogField{Key: "product_id", Value: productID},
				interfaces.LogField{Key: "quantity", Value: quantity},
			)

			cacheKey := fmt.Sprintf("product:%s", productID)
			productService.InvalidateCache(evtCtx, cacheKey, event.TenantID)

		case "product_synced_to_marketplace":
			productID, _ := event.Payload["product_id"].(string)
			marketplaceID, _ := event.Payload["marketplace_id"].(float64)
			status, _ := event.Payload["status"].(string)

			log.InfoWithContext(evtCtx, "Продукт синхронизирован с маркетплейсом",
				interfaces.LogField{Key: "product_id", Value: productID},
				interfaces.LogField{Key: "marketplace_id", Value: marketplaceID},
				interfaces.LogField{Key: "status", Value: status},
			)

			if status == "success" {
				product, err := productService.GetProduct(evtCtx, productID, event.SupplierID, event.TenantID)
				if err == nil && product != nil {
					// Обновляем метаданные и сохраняем продукт
					// ...
				}
			}

		default:
			log.WarnWithContext(ctx, "Неизвестный тип события",
				interfaces.LogField{Key: "event_type", Value: event.EventType},
				interfaces.LogField{Key: "message_id", Value: msg.ID},
			)
			messagesProcessed.WithLabelValues(msg.Topic, "unknown").Inc()
			return nil
		}

		duration := time.Since(startTime).Seconds()
		messageProcessingDuration.WithLabelValues(msg.Topic).Observe(duration)
		messagesProcessed.WithLabelValues(msg.Topic, "success").Inc()

		log.InfoWithContext(evtCtx, "Событие успешно обработано",
			interfaces.LogField{Key: "event_type", Value: event.EventType},
			interfaces.LogField{Key: "message_id", Value: msg.ID},
			interfaces.LogField{Key: "duration", Value: duration},
		)

		return nil
	}

	wg.Add(1)

	go func() {
		defer wg.Done()

		unsubscribe, err := messagingClient.Subscribe(ctx, "product-events", eventHandler)
		if err != nil {
			log.Error("Ошибка подписки на события продуктов",
				interfaces.LogField{Key: "error", Value: err.Error()})
			return
		}
		defer unsubscribe()

		log.Info("Подписка на события продуктов установлена")

		<-ctx.Done()
		log.Info("Отмена подписки на события продуктов")
	}()
}
