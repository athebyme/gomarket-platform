package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/google/uuid"
	"strings"
	"sync"
	"time"
)

// KafkaConfig представляет конфигурацию Kafka клиента
type KafkaConfig struct {
	Brokers          []string
	GroupID          string
	DeadLetterTopic  string
	SessionTimeout   time.Duration
	HeartbeatTimeout time.Duration
	MaxRetries       int
	RetryBackoff     time.Duration
}

type KafkaMessaging struct {
	producer         *kafka.Producer
	consumers        map[string]*kafka.Consumer
	consumersMutex   sync.RWMutex
	handlers         map[string]interfaces.MessageHandler
	handlersMutex    sync.RWMutex
	brokers          []string
	groupID          string
	deadLetterTopic  string
	logger           interfaces.LoggerPort
	consumerContexts map[string]context.CancelFunc
	contextsMutex    sync.RWMutex
}

func NewKafkaMessaging(
	brokers []string,
	groupID string,
	deadLetterTopic string,
	logger interfaces.LoggerPort,
) (interfaces.MessagingPort, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf("не указаны брокеры Kafka")
	}
	if groupID == "" {
		return nil, fmt.Errorf("не указан GroupID")
	}

	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(brokers, ","),
		"client.id":          "product-service-producer-" + uuid.New().String()[:8],
		"acks":               "all",    // максимальная надежность
		"retries":            5,        // количество повторных попыток
		"retry.backoff.ms":   500,      // задержка между повторами
		"compression.type":   "snappy", // алгоритм сжатия для экономии трафика
		"linger.ms":          10,       // задержка для батчинга сообщений
		"batch.size":         16384,    // размер батча в байтах
		"enable.idempotence": true,     // гарантия "exactly once" доставки
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Kafka producer: %w", err)
	}

	go func() {
		for e := range producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					logger.Error("Ошибка доставки сообщения в Kafka",
						interfaces.LogField{Key: "topic", Value: *ev.TopicPartition.Topic},
						interfaces.LogField{Key: "error", Value: ev.TopicPartition.Error.Error()},
					)
				} else {
					logger.Debug("Сообщение успешно доставлено в Kafka",
						interfaces.LogField{Key: "topic", Value: *ev.TopicPartition.Topic},
						interfaces.LogField{Key: "partition", Value: ev.TopicPartition.Partition},
						interfaces.LogField{Key: "offset", Value: ev.TopicPartition.Offset},
					)
				}
			}
		}
	}()

	return &KafkaMessaging{
		producer:         producer,
		consumers:        make(map[string]*kafka.Consumer),
		consumersMutex:   sync.RWMutex{},
		handlers:         make(map[string]interfaces.MessageHandler),
		handlersMutex:    sync.RWMutex{},
		brokers:          brokers,
		groupID:          groupID,
		deadLetterTopic:  deadLetterTopic,
		logger:           logger,
		consumerContexts: make(map[string]context.CancelFunc),
		contextsMutex:    sync.RWMutex{},
	}, nil
}

// Publish публикует сообщение в топик
func (k *KafkaMessaging) Publish(ctx context.Context, topic string, message []byte) error {
	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          message,
		Headers: []kafka.Header{
			{Key: "message_id", Value: []byte(uuid.New().String())},
			{Key: "timestamp", Value: []byte(fmt.Sprintf("%d", time.Now().UnixNano()))},
		},
	}

	if tenantID, ok := ctx.Value("tenant_id").(string); ok && tenantID != "" {
		msg.Headers = append(msg.Headers, kafka.Header{Key: "tenant_id", Value: []byte(tenantID)})
	}

	if traceID, ok := ctx.Value("trace_id").(string); ok && traceID != "" {
		msg.Headers = append(msg.Headers, kafka.Header{Key: "trace_id", Value: []byte(traceID)})
	}

	err := k.producer.Produce(msg, nil)
	if err != nil {
		return fmt.Errorf("ошибка отправки сообщения в Kafka: %w", err)
	}

	return nil
}

func (k *KafkaMessaging) Subscribe(ctx context.Context, topic string, handler interfaces.MessageHandler) (func() error, error) {
	consumerID := uuid.New().String()

	consumerCtx, cancel := context.WithCancel(context.Background())

	k.contextsMutex.Lock()
	k.consumerContexts[consumerID] = cancel
	k.contextsMutex.Unlock()

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":       strings.Join(k.brokers, ","),
		"group.id":                k.groupID,
		"auto.offset.reset":       "latest",
		"enable.auto.commit":      true,
		"auto.commit.interval.ms": 5000,
		"session.timeout.ms":      30000,
		"max.poll.interval.ms":    300000,
		"heartbeat.interval.ms":   10000,
		"fetch.min.bytes":         1,
		"fetch.max.bytes":         52428800, // 50MB
		"fetch.max.wait.ms":       500,
		"isolation.level":         "read_committed", // чтение только подтвержденных транзакций
	})
	if err != nil {
		k.contextsMutex.Lock()
		delete(k.consumerContexts, consumerID)
		k.contextsMutex.Unlock()
		cancel()
		return nil, fmt.Errorf("ошибка создания Kafka consumer: %w", err)
	}

	err = consumer.Subscribe(topic, nil)
	if err != nil {
		consumer.Close()
		k.contextsMutex.Lock()
		delete(k.consumerContexts, consumerID)
		k.contextsMutex.Unlock()
		cancel()
		return nil, fmt.Errorf("ошибка подписки на топик %s: %w", topic, err)
	}

	k.consumersMutex.Lock()
	k.consumers[consumerID] = consumer
	k.consumersMutex.Unlock()

	k.handlersMutex.Lock()
	k.handlers[consumerID] = handler
	k.handlersMutex.Unlock()

	go k.consumeMessages(consumerCtx, consumer, consumerID)

	unsubscribe := func() error {
		k.contextsMutex.Lock()
		if cancelFunc, exists := k.consumerContexts[consumerID]; exists {
			cancelFunc()
			delete(k.consumerContexts, consumerID)
		}
		k.contextsMutex.Unlock()

		k.handlersMutex.Lock()
		delete(k.handlers, consumerID)
		k.handlersMutex.Unlock()

		k.consumersMutex.Lock()
		if c, exists := k.consumers[consumerID]; exists {
			delete(k.consumers, consumerID)
			if err := c.Close(); err != nil {
				return fmt.Errorf("ошибка закрытия consumer: %w", err)
			}
		}
		k.consumersMutex.Unlock()

		return nil
	}

	return unsubscribe, nil
}

func (k *KafkaMessaging) consumeMessages(ctx context.Context, consumer *kafka.Consumer, consumerID string) {
	const maxRetries = 3

	for {
		select {
		case <-ctx.Done():
			k.logger.Info("Завершение обработки сообщений для consumer",
				interfaces.LogField{Key: "consumer_id", Value: consumerID})
			return

		default:
			ev := consumer.Poll(100)
			if ev == nil {
				continue
			}

			switch e := ev.(type) {
			case *kafka.Message:
				k.handlersMutex.RLock()
				handler, ok := k.handlers[consumerID]
				k.handlersMutex.RUnlock()

				if !ok {
					k.logger.Warn("Handler не найден для consumer",
						interfaces.LogField{Key: "consumer_id", Value: consumerID})
					continue
				}

				msg := k.kafkaToInterfaceMessage(e)

				var processingErr error

				for attempt := 0; attempt < maxRetries; attempt++ {
					msg.Attempts++

					msgCtx := ctx
					if msg.TenantID != "" {
						msgCtx = context.WithValue(ctx, "tenant_id", msg.TenantID)
					}

					if traceID, ok := msg.Headers["trace_id"]; ok {
						msgCtx = context.WithValue(msgCtx, "trace_id", traceID)
					}

					processingErr = handler(msgCtx, msg)
					if processingErr == nil {
						break
					}

					k.logger.WarnWithContext(msgCtx, "Ошибка обработки сообщения, повторная попытка",
						interfaces.LogField{Key: "topic", Value: msg.Topic},
						interfaces.LogField{Key: "message_id", Value: msg.ID},
						interfaces.LogField{Key: "attempt", Value: attempt + 1},
						interfaces.LogField{Key: "error", Value: processingErr.Error()},
					)

					backoff := time.Duration(2^attempt) * 100 * time.Millisecond
					time.Sleep(backoff)
				}

				if processingErr != nil && k.deadLetterTopic != "" {
					k.sendToDLQ(ctx, msg, processingErr.Error(), maxRetries)
				}

			case kafka.Error:
				// Обработка ошибок Kafka
				if e.Code() == kafka.ErrAllBrokersDown {
					k.logger.Error("Все брокеры Kafka недоступны, прекращение обработки",
						interfaces.LogField{Key: "error", Value: e.Error()},
						interfaces.LogField{Key: "consumer_id", Value: consumerID},
					)
					return
				}

				k.logger.Error("Ошибка Kafka",
					interfaces.LogField{Key: "error", Value: e.Error()},
					interfaces.LogField{Key: "code", Value: e.Code()},
					interfaces.LogField{Key: "consumer_id", Value: consumerID},
				)
			}
		}
	}
}

// sendToDLQ отправляет сообщение в Dead Letter Queue
func (k *KafkaMessaging) sendToDLQ(ctx context.Context, originalMsg *interfaces.Message, errorMsg string, retryCount int) {
	dlqMessage := struct {
		OriginalMessage *interfaces.Message `json:"original_message"`
		Error           string              `json:"error"`
		RetryCount      int                 `json:"retry_count"`
		Timestamp       time.Time           `json:"timestamp"`
	}{
		OriginalMessage: originalMsg,
		Error:           errorMsg,
		RetryCount:      retryCount,
		Timestamp:       time.Now().UTC(),
	}

	dlqData, err := json.Marshal(dlqMessage)
	if err != nil {
		k.logger.Error("Ошибка сериализации сообщения для DLQ",
			interfaces.LogField{Key: "error", Value: err.Error()},
			interfaces.LogField{Key: "message_id", Value: originalMsg.ID},
		)
		return
	}

	err = k.Publish(ctx, k.deadLetterTopic, dlqData)
	if err != nil {
		k.logger.Error("Ошибка отправки сообщения в DLQ",
			interfaces.LogField{Key: "error", Value: err.Error()},
			interfaces.LogField{Key: "message_id", Value: originalMsg.ID},
		)
		return
	}

	k.logger.Info("Сообщение отправлено в DLQ",
		interfaces.LogField{Key: "message_id", Value: originalMsg.ID},
		interfaces.LogField{Key: "topic", Value: originalMsg.Topic},
		interfaces.LogField{Key: "error", Value: errorMsg},
	)
}

// kafkaToInterfaceMessage преобразует Kafka сообщение в интерфейсное
func (k *KafkaMessaging) kafkaToInterfaceMessage(msg *kafka.Message) *interfaces.Message {
	headers := make(map[string]string)
	for _, header := range msg.Headers {
		headers[header.Key] = string(header.Value)
	}

	var key string
	if msg.Key != nil {
		key = string(msg.Key)
	}

	return &interfaces.Message{
		ID:          headers["message_id"],
		Topic:       *msg.TopicPartition.Topic,
		Key:         key,
		Value:       msg.Value,
		Headers:     headers,
		Metadata:    make(map[string]interface{}),
		TenantID:    headers["tenant_id"],
		PublishedAt: time.Now(),
		Attempts:    0,
	}
}

// Close закрывает соединения с Kafka
func (k *KafkaMessaging) Close() error {
	k.contextsMutex.Lock()
	for _, cancel := range k.consumerContexts {
		cancel()
	}
	k.consumerContexts = make(map[string]context.CancelFunc)
	k.contextsMutex.Unlock()

	k.consumersMutex.Lock()
	for id, consumer := range k.consumers {
		if err := consumer.Close(); err != nil {
			k.logger.Error("Ошибка закрытия consumer",
				interfaces.LogField{Key: "error", Value: err.Error()},
				interfaces.LogField{Key: "consumer_id", Value: id},
			)
		}
	}
	k.consumers = make(map[string]*kafka.Consumer)
	k.consumersMutex.Unlock()

	k.handlersMutex.Lock()
	k.handlers = make(map[string]interfaces.MessageHandler)
	k.handlersMutex.Unlock()

	timeoutMS := 5000
	k.logger.Info("Ожидание отправки всех сообщений в Kafka",
		interfaces.LogField{Key: "timeout_ms", Value: timeoutMS},
	)
	k.producer.Flush(timeoutMS)
	k.producer.Close()

	return nil
}
