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
	// Загружаем конфигурацию
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("Ошибка загрузки конфигурации: %v\n", err)
		os.Exit(1)
	}

	// Создаем корневой контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализируем логгер
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

	db, err := postgres.NewPostgresRepository(
		ctx,
		connectionStr,
	)
	if err != nil {
		log.Fatal("Ошибка инициализации хранилища", interfaces.LogField{Key: "error", Value: err.Error()})
	}
	defer db.Close()

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

// subscribeToProductCommands подписывается на команды для продуктов
func subscribeToProductCommands(
	ctx context.Context,
	messagingClient interfaces.MessagingPort,
	productService *services.ProductService,
	log interfaces.LoggerPort,
	wg *sync.WaitGroup,
) {
	commandHandler := func(ctx context.Context, msg *interfaces.Message) error {
		startTime := time.Now()
		activeWorkers.Inc()
		defer activeWorkers.Dec()

		log.InfoWithContext(ctx, "Получена команда для продукта",
			interfaces.LogField{Key: "message_id", Value: msg.ID},
			interfaces.LogField{Key: "topic", Value: msg.Topic},
		)

		var command struct {
			CommandType string                 `json:"command_type"`
			TenantID    string                 `json:"tenant_id"`
			Payload     map[string]interface{} `json:"payload"`
		}

		if err := json.Unmarshal(msg.Value, &command); err != nil {
			log.ErrorWithContext(ctx, "Ошибка декодирования команды",
				interfaces.LogField{Key: "error", Value: err.Error()},
				interfaces.LogField{Key: "message_id", Value: msg.ID},
			)
			messagesProcessed.WithLabelValues(msg.Topic, "error").Inc()
			return err
		}

		cmdCtx := context.WithValue(ctx, "tenant_id", command.TenantID)

		var err error
		switch command.CommandType {
		case "sync_products_from_supplier":
			// Пример обработки команды синхронизации продуктов от поставщика
			supplierID, ok := command.Payload["supplier_id"].(float64)
			if !ok {
				log.ErrorWithContext(ctx, "Некорректный формат supplier_id",
					interfaces.LogField{Key: "message_id", Value: msg.ID},
				)
				messagesProcessed.WithLabelValues(msg.Topic, "error").Inc()
				return fmt.Errorf("некорректный формат supplier_id")
			}

			_, err = productService.SyncProductsFromSupplier(cmdCtx, int(supplierID), command.TenantID)

		case "sync_product_to_marketplace":
			productID, ok := command.Payload["product_id"].(string)
			if !ok {
				log.ErrorWithContext(ctx, "Некорректный формат product_id",
					interfaces.LogField{Key: "message_id", Value: msg.ID},
				)
				messagesProcessed.WithLabelValues(msg.Topic, "error").Inc()
				return fmt.Errorf("некорректный формат product_id")
			}

			marketplaceID, ok := command.Payload["marketplace_id"].(float64)
			if !ok {
				log.ErrorWithContext(ctx, "Некорректный формат marketplace_id",
					interfaces.LogField{Key: "message_id", Value: msg.ID},
				)
				messagesProcessed.WithLabelValues(msg.Topic, "error").Inc()
				return fmt.Errorf("некорректный формат marketplace_id")
			}

			err = productService.SyncProductToMarketplace(cmdCtx, productID, int(marketplaceID), command.TenantID)

		default:
			log.WarnWithContext(ctx, "Неизвестный тип команды",
				interfaces.LogField{Key: "command_type", Value: command.CommandType},
				interfaces.LogField{Key: "message_id", Value: msg.ID},
			)
			messagesProcessed.WithLabelValues(msg.Topic, "unknown").Inc()
			return nil // Игнорируем неизвестные команды
		}

		if err != nil {
			log.ErrorWithContext(ctx, "Ошибка выполнения команды",
				interfaces.LogField{Key: "error", Value: err.Error()},
				interfaces.LogField{Key: "command_type", Value: command.CommandType},
				interfaces.LogField{Key: "message_id", Value: msg.ID},
			)
			messagesProcessed.WithLabelValues(msg.Topic, "error").Inc()
			return err
		}

		// Обновляем метрики
		duration := time.Since(startTime).Seconds()
		messageProcessingDuration.WithLabelValues(msg.Topic).Observe(duration)
		messagesProcessed.WithLabelValues(msg.Topic, "success").Inc()

		log.InfoWithContext(ctx, "Команда успешно обработана",
			interfaces.LogField{Key: "command_type", Value: command.CommandType},
			interfaces.LogField{Key: "message_id", Value: msg.ID},
			interfaces.LogField{Key: "duration", Value: duration},
		)

		return nil
	}

	cfg := &interfaces.ConsumerConfig{
		GroupID:            "product-service-commands",
		AutoCommit:         false, // Ручное подтверждение для гарантии обработки
		AutoCommitInterval: 0,
		MaxPollRecords:     10,
		PollTimeout:        100 * time.Millisecond,
	}

	// Увеличиваем счетчик в WaitGroup
	wg.Add(1)

	// Подписываемся на топик команд
	go func() {
		defer wg.Done()

		// Подписываемся на топик команд
		unsubscribe, err := messagingClient.SubscribeWithConfig(ctx, "product-commands", commandHandler, cfg)
		if err != nil {
			log.Error("Ошибка подписки на команды продуктов",
				interfaces.LogField{Key: "error", Value: err.Error()})
			return
		}
		defer unsubscribe()

		log.Info("Подписка на команды продуктов установлена")

		// Ожидаем отмены контекста
		<-ctx.Done()
		log.Info("Отмена подписки на команды продуктов")
	}()
}

// subscribeToProductEvents подписывается на события продуктов
func subscribeToProductEvents(
	ctx context.Context,
	messagingClient interfaces.MessagingPort,
	productService *services.ProductService,
	log interfaces.LoggerPort,
	wg *sync.WaitGroup,
) {
	// Обработчик событий для продуктов
	eventHandler := func(ctx context.Context, msg *interfaces.Message) error {
		startTime := time.Now()
		activeWorkers.Inc()
		defer activeWorkers.Dec()

		log.InfoWithContext(ctx, "Получено событие продукта",
			interfaces.LogField{Key: "message_id", Value: msg.ID},
			interfaces.LogField{Key: "topic", Value: msg.Topic},
		)

		// Декодируем событие
		var event struct {
			EventType string                 `json:"event_type"`
			TenantID  string                 `json:"tenant_id"`
			Payload   map[string]interface{} `json:"payload"`
		}

		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.ErrorWithContext(ctx, "Ошибка декодирования события",
				interfaces.LogField{Key: "error", Value: err.Error()},
				interfaces.LogField{Key: "message_id", Value: msg.ID},
			)
			messagesProcessed.WithLabelValues(msg.Topic, "error").Inc()
			return err
		}

		// Создаем контекст с tenant_id
		evtCtx := context.WithValue(ctx, "tenant_id", event.TenantID)

		// Обрабатываем событие в зависимости от его типа
		switch event.EventType {
		case "product_price_updated":
			// Пример обработки события обновления цены продукта
			// В реальном приложении здесь был бы код для обработки события

		case "product_inventory_updated":
			// Пример обработки события обновления инвентаря продукта
			// В реальном приложении здесь был бы код для обработки события

		default:
			log.WarnWithContext(ctx, "Неизвестный тип события",
				interfaces.LogField{Key: "event_type", Value: event.EventType},
				interfaces.LogField{Key: "message_id", Value: msg.ID},
			)
			messagesProcessed.WithLabelValues(msg.Topic, "unknown").Inc()
			return nil // Игнорируем неизвестные события
		}

		// Обновляем метрики
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

	// Подписываемся на события с настройками
	config := &interfaces.ConsumerConfig{
		GroupID:            "product-service-events",
		AutoCommit:         true, // Автоматическое подтверждение
		AutoCommitInterval: 5 * time.Second,
		MaxPollRecords:     100,
		PollTimeout:        100 * time.Millisecond,
	}

	// Увеличиваем счетчик в WaitGroup
	wg.Add(1)

	// Подписываемся на топик событий
	go func() {
		defer wg.Done()

		// Подписываемся на топик событий
		unsubscribe, err := messagingClient.SubscribeWithConfig(ctx, "product-events", eventHandler, config)
		if err != nil {
			log.Error("Ошибка подписки на события продуктов",
				interfaces.LogField{Key: "error", Value: err.Error()})
			return
		}
		defer unsubscribe()

		log.Info("Подписка на события продуктов установлена")

		// Ожидаем отмены контекста
		<-ctx.Done()
		log.Info("Отмена подписки на события продуктов")
	}()
}
