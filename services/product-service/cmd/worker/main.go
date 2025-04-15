package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Метрики для Prometheus
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

	// Запускаем HTTP сервер для метрик если они включены
	if cfg.Metrics.Enabled {
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			addr := fmt.Sprintf(":%d", cfg.Metrics.Port)
			log.Info("Запуск HTTP сервера для метрик",
				interfaces.LogField{Key: "addr", Value: addr})

			if err := http.ListenAndServe(addr, mux); err != nil {
				log.Error("Ошибка запуска HTTP сервера для метрик",
					interfaces.LogField{Key: "error", Value: err.Error()})
			}
		}()
	}

	// Генерируем строку подключения к PostgreSQL
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
	if err != nil {
		log.Fatal("Ошибка генерации строки подключения к PostgreSQL",
			interfaces.LogField{Key: "error", Value: err.Error()})
	}

	// Инициализируем хранилище
	repo, err := postgres.NewPostgresStorage(ctx, connectionStr)
	if err != nil {
		log.Fatal("Ошибка инициализации хранилища",
			interfaces.LogField{Key: "error", Value: err.Error()})
	}
	defer repo.Close()
	log.Info("Хранилище инициализировано")

	// Инициализируем кэш
	cacheClient, err := cache.NewRedisCache(
		ctx,
		cfg.Redis.Host,
		cfg.Redis.Port,
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	if err != nil {
		log.Fatal("Ошибка инициализации кэша",
			interfaces.LogField{Key: "error", Value: err.Error()})
	}
	defer cacheClient.Close()
	log.Info("Кэш инициализирован")

	// Инициализируем систему обмена сообщениями
	messagingClient, err := messaging.NewKafkaMessaging(
		cfg.Kafka.Brokers,
		cfg.Kafka.GroupID,
		cfg.Kafka.DeadLetterTopic,
		log,
	)
	if err != nil {
		log.Fatal("Ошибка инициализации системы обмена сообщениями",
			interfaces.LogField{Key: "error", Value: err.Error()})
	}
	defer messagingClient.Close()
	log.Info("Система обмена сообщениями инициализирована")

	// Инициализируем сервис продуктов
	productService := services.NewProductService(repo, cacheClient, messagingClient, log)
	log.Info("Сервис продуктов инициализирован")

	// Каналы для сигналов и завершения
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Подписываемся на команды и события
	subscribeToProductCommands(ctx, messagingClient, productService, log, &wg)
	subscribeToProductEvents(ctx, messagingClient, productService, log, &wg)

	// Обработка сигналов завершения
	go func() {
		<-quit
		log.Info("Получен сигнал завершения, выполняется graceful shutdown...")
		cancel()
		wg.Wait()
		close(done)
	}()

	log.Info("Воркер запущен и готов к обработке сообщений")
	<-done
	log.Info("Воркер корректно завершил работу")
}

// Подписка на команды продуктов
func subscribeToProductCommands(ctx context.Context, messagingClient interfaces.MessagingPort,
	productService services.ProductServiceInterface,
	logger interfaces.LoggerPort, wg *sync.WaitGroup) {

	commandHandler := func(ctx context.Context, msg *interfaces.Message) error {
		startTime := time.Now()
		activeWorkers.Inc()
		defer activeWorkers.Dec()

		logger.InfoWithContext(ctx, "Получена команда продукта",
			interfaces.LogField{Key: "message_id", Value: msg.ID},
			interfaces.LogField{Key: "topic", Value: msg.Topic},
		)

		var command struct {
			CommandType string                 `json:"command_type"`
			TenantID    string                 `json:"tenant_id"`
			ProductID   string                 `json:"product_id"`
			Payload     map[string]interface{} `json:"payload"`
		}

		if err := json.Unmarshal(msg.Value, &command); err != nil {
			logger.ErrorWithContext(ctx, "Ошибка декодирования команды",
				interfaces.LogField{Key: "error", Value: err.Error()})
			messagesProcessed.WithLabelValues(msg.Topic, "error").Inc()
			return err
		}

		// Добавляем tenant_id в контекст
		cmdCtx := context.WithValue(ctx, "tenant_id", command.TenantID)
		var err error

		// Обрабатываем команду в зависимости от типа
		switch command.CommandType {
		case "sync_product":
			marketplaceID, ok := command.Payload["marketplace_id"].(float64)
			if !ok {
				err = fmt.Errorf("неверный формат marketplace_id")
				break
			}
			err = productService.SyncProductToMarketplace(cmdCtx, command.ProductID, int(marketplaceID), command.TenantID)

		case "sync_supplier":
			supplierID, ok := command.Payload["supplier_id"].(float64)
			if !ok {
				err = fmt.Errorf("неверный формат supplier_id")
				break
			}
			_, err = productService.SyncProductsFromSupplier(cmdCtx, int(supplierID), command.TenantID)

		case "invalidate_cache":
			cacheKey := fmt.Sprintf("product:%s", command.ProductID)
			err = productService.InvalidateCache(cmdCtx, cacheKey, command.TenantID)

		default:
			logger.WarnWithContext(ctx, "Неизвестный тип команды",
				interfaces.LogField{Key: "command_type", Value: command.CommandType})
			messagesProcessed.WithLabelValues(msg.Topic, "unknown").Inc()
			return nil
		}

		if err != nil {
			logger.ErrorWithContext(cmdCtx, "Ошибка обработки команды",
				interfaces.LogField{Key: "error", Value: err.Error()})
			messagesProcessed.WithLabelValues(msg.Topic, "error").Inc()
			return err
		}

		duration := time.Since(startTime).Seconds()
		messageProcessingDuration.WithLabelValues(msg.Topic).Observe(duration)
		messagesProcessed.WithLabelValues(msg.Topic, "success").Inc()

		logger.InfoWithContext(cmdCtx, "Команда успешно обработана",
			interfaces.LogField{Key: "command_type", Value: command.CommandType},
			interfaces.LogField{Key: "duration", Value: duration},
		)

		return nil
	}

	wg.Add(1)

	go func() {
		defer wg.Done()

		unsubscribe, err := messagingClient.Subscribe(ctx, "product-commands", commandHandler)
		if err != nil {
			logger.Error("Ошибка подписки на команды продуктов",
				interfaces.LogField{Key: "error", Value: err.Error()})
			return
		}
		defer unsubscribe()

		logger.Info("Подписка на команды продуктов установлена")

		<-ctx.Done()
		logger.Info("Отмена подписки на команды продуктов")
	}()
}

// Подписка на события продуктов
func subscribeToProductEvents(ctx context.Context, messagingClient interfaces.MessagingPort,
	productService services.ProductServiceInterface,
	logger interfaces.LoggerPort, wg *sync.WaitGroup) {

	eventHandler := func(ctx context.Context, msg *interfaces.Message) error {
		startTime := time.Now()
		activeWorkers.Inc()
		defer activeWorkers.Dec()

		logger.InfoWithContext(ctx, "Получено событие продукта",
			interfaces.LogField{Key: "message_id", Value: msg.ID},
			interfaces.LogField{Key: "topic", Value: msg.Topic},
		)

		var event struct {
			EventType  string                 `json:"event_type"`
			TenantID   string                 `json:"tenant_id"`
			ProductID  string                 `json:"product_id"`
			SupplierID string                 `json:"supplier_id,omitempty"`
			Payload    map[string]interface{} `json:"payload,omitempty"`
		}

		if err := json.Unmarshal(msg.Value, &event); err != nil {
			logger.ErrorWithContext(ctx, "Ошибка декодирования события",
				interfaces.LogField{Key: "error", Value: err.Error()},
				interfaces.LogField{Key: "message_id", Value: msg.ID},
			)
			messagesProcessed.WithLabelValues(msg.Topic, "error").Inc()
			return err
		}

		// Добавляем tenant_id в контекст
		evtCtx := context.WithValue(ctx, "tenant_id", event.TenantID)

		// Обработка события в зависимости от типа
		switch event.EventType {
		case messaging.ProductCreatedEvent:
			// Логика обработки события создания продукта
			logger.InfoWithContext(evtCtx, "Обработка события создания продукта",
				interfaces.LogField{Key: "product_id", Value: event.ProductID},
			)

		case messaging.ProductUpdatedEvent:
			// Логика обработки события обновления продукта
			logger.InfoWithContext(evtCtx, "Обработка события обновления продукта",
				interfaces.LogField{Key: "product_id", Value: event.ProductID},
			)

			// Инвалидация кэша для обновленного продукта
			cacheKey := fmt.Sprintf("product:%s", event.ProductID)
			_ = productService.InvalidateCache(evtCtx, cacheKey, event.TenantID)

		case messaging.ProductDeletedEvent:
			// Логика обработки события удаления продукта
			logger.InfoWithContext(evtCtx, "Обработка события удаления продукта",
				interfaces.LogField{Key: "product_id", Value: event.ProductID},
			)

			// Инвалидация кэша для удаленного продукта
			cacheKey := fmt.Sprintf("product:%s", event.ProductID)
			_ = productService.InvalidateCache(evtCtx, cacheKey, event.TenantID)

		case "product_price_updated":
			// Обработка события обновления цены
			productID, _ := event.Payload["product_id"].(string)
			price, _ := event.Payload["price"].(float64)

			logger.InfoWithContext(evtCtx, "Обработка события обновления цены",
				interfaces.LogField{Key: "product_id", Value: productID},
				interfaces.LogField{Key: "price", Value: price},
			)

			cacheKey := fmt.Sprintf("product:%s", productID)
			_ = productService.InvalidateCache(evtCtx, cacheKey, event.TenantID)

		case "product_inventory_updated":
			// Обработка события обновления инвентаря
			productID, _ := event.Payload["product_id"].(string)
			quantity, _ := event.Payload["quantity"].(float64)

			logger.InfoWithContext(evtCtx, "Обработка события обновления инвентаря",
				interfaces.LogField{Key: "product_id", Value: productID},
				interfaces.LogField{Key: "quantity", Value: quantity},
			)

			cacheKey := fmt.Sprintf("product:%s", productID)
			_ = productService.InvalidateCache(evtCtx, cacheKey, event.TenantID)

		default:
			logger.WarnWithContext(ctx, "Неизвестный тип события",
				interfaces.LogField{Key: "event_type", Value: event.EventType},
			)
			messagesProcessed.WithLabelValues(msg.Topic, "unknown").Inc()
			return nil
		}

		// Метрики успешной обработки
		duration := time.Since(startTime).Seconds()
		messageProcessingDuration.WithLabelValues(msg.Topic).Observe(duration)
		messagesProcessed.WithLabelValues(msg.Topic, "success").Inc()

		logger.InfoWithContext(evtCtx, "Событие успешно обработано",
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
			logger.Error("Ошибка подписки на события продуктов",
				interfaces.LogField{Key: "error", Value: err.Error()})
			return
		}
		defer unsubscribe()

		logger.Info("Подписка на события продуктов установлена")

		<-ctx.Done()
		logger.Info("Отмена подписки на события продуктов")
	}()
}
