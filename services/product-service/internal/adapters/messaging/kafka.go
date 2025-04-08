package messaging

import (
	"context"
	"fmt"
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/google/uuid"
	"sync"
	"time"
)

// KafkaMessaging реализация MessagingPort с использованием Kafka
type KafkaMessaging struct {
	producer       *kafka.Producer
	consumers      map[string]*kafka.Consumer
	consumersMutex sync.RWMutex
	handlers       map[string]map[string]interfaces.MessageHandler // topic -> handlerID -> handler
	handlersMutex  sync.RWMutex
	brokers        []string
	groupID        string
}

// NewKafkaMessaging создает новый экземпляр KafkaMessaging
func NewKafkaMessaging(brokers []string, groupID string) (interfaces.MessagingPort, error) {
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":            brokers,
		"client.id":                    "product-service-producer",
		"acks":                         "all", // максимальная надежность
		"retries":                      5,
		"retry.backoff.ms":             500,
		"compression.type":             "snappy",
		"linger.ms":                    10,    // небольшая задержка для батчинга
		"batch.size":                   16384, // размер пакета в байтах
		"message.max.bytes":            1000000,
		"queue.buffering.max.messages": 100000, // размер внутреннего буфера
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Kafka producer: %w", err)
	}

	return &KafkaMessaging{
		producer:       producer,
		consumers:      make(map[string]*kafka.Consumer),
		consumersMutex: sync.RWMutex{},
		handlers:       make(map[string]map[string]interfaces.MessageHandler),
		handlersMutex:  sync.RWMutex{},
		brokers:        brokers,
		groupID:        groupID,
	}, nil
}

// messageToKafkaMessage преобразует Message в kafka.Message
func messageToKafkaMessage(topic string, message []byte, key string, headers map[string]string) *kafka.Message {
	var kafkaHeaders []kafka.Header
	for k, v := range headers {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	// Добавляем служебные заголовки
	kafkaHeaders = append(kafkaHeaders,
		kafka.Header{Key: "message_id", Value: []byte(uuid.New().String())},
		kafka.Header{Key: "timestamp", Value: []byte(fmt.Sprintf("%d", time.Now().UnixNano()))},
	)

	var keyBytes []byte
	if key != "" {
		keyBytes = []byte(key)
	}

	return &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          message,
		Key:            keyBytes,
		Headers:        kafkaHeaders,
	}
}

// kafkaMessageToMessage преобразует kafka.Message в Message
func kafkaMessageToMessage(msg *kafka.Message) *interfaces.Message {
	headers := make(map[string]string)
	for _, header := range msg.Headers {
		headers[header.Key] = string(header.Value)
	}

	var key string
	if msg.Key != nil {
		key = string(msg.Key)
	}

	// Извлекаем tenant_id из заголовков, если есть
	tenantID := headers["tenant_id"]

	// Извлекаем время публикации из заголовков
	publishedAt := time.Now()
	if tsStr, ok := headers["timestamp"]; ok {
		if ts, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
			publishedAt = ts
		}
	}

	return &interfaces.Message{
		ID:          headers["message_id"],
		Topic:       *msg.TopicPartition.Topic,
		Key:         key,
		Value:       msg.Value,
		Headers:     headers,
		Metadata:    make(map[string]interface{}),
		TenantID:    tenantID,
		PublishedAt: publishedAt,
		Attempts:    0,
	}
}

// Publish публикует сообщение в указанную тему
func (k *KafkaMessaging) Publish(ctx context.Context, topic string, message []byte) error {
	msg := messageToKafkaMessage(topic, message, "", nil)
	return k.producer.Produce(msg, nil)
}

// PublishWithKey публикует сообщение с указанным ключом
func (k *KafkaMessaging) PublishWithKey(ctx context.Context, topic string, key string, message []byte) error {
	msg := messageToKafkaMessage(topic, message, key, nil)
	return k.producer.Produce(msg, nil)
}

// PublishWithHeaders публикует сообщение с дополнительными заголовками
func (k *KafkaMessaging) PublishWithHeaders(ctx context.Context, topic string, message []byte, headers map[string]string) error {
	msg := messageToKafkaMessage(topic, message, "", headers)
	return k.producer.Produce(msg, nil)
}

// PublishForTenant публикует сообщение с учетом ID арендатора
func (k *KafkaMessaging) PublishForTenant(ctx context.Context, topic string, message []byte, tenantID string) error {
	headers := map[string]string{"tenant_id": tenantID}
	return k.PublishWithHeaders(ctx, topic, message, headers)
}

// PublishBatch публикует несколько сообщений одним запросом
func (k *KafkaMessaging) PublishBatch(ctx context.Context, topic string, messages [][]byte) error {
	for _, message := range messages {
		if err := k.Publish(ctx, topic, message); err != nil {
			return err
		}
	}
	return nil
}

// PublishBatchWithKeys публикует несколько сообщений с ключами одним запросом
func (k *KafkaMessaging) PublishBatchWithKeys(ctx context.Context, topic string, keyedMessages map[string][]byte) error {
	for key, message := range keyedMessages {
		if err := k.PublishWithKey(ctx, topic, key, message); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe подписывается на указанную тему и обрабатывает сообщения с помощью handler
func (k *KafkaMessaging) Subscribe(ctx context.Context, topic string, handler interfaces.MessageHandler) (func() error, error) {
	config := &interfaces.ConsumerConfig{
		GroupID:            k.groupID,
		AutoCommit:         true,
		AutoCommitInterval: 5 * time.Second,
		MaxPollRecords:     500,
		PollTimeout:        100 * time.Millisecond,
	}
	return k.SubscribeWithConfig(ctx, topic, handler, config)
}

// SubscribeWithConfig подписывается на указанную тему с дополнительными настройками
func (k *KafkaMessaging) SubscribeWithConfig(ctx context.Context, topic string, handler interfaces.MessageHandler, config *interfaces.ConsumerConfig) (func() error, error) {
	// Создаем уникальный ID для обработчика
	handlerID := uuid.New().String()

	// Регистрируем обработчик
	k.handlersMutex.Lock()
	if _, ok := k.handlers[topic]; !ok {
		k.handlers[topic] = make(map[string]interfaces.MessageHandler)
	}
	k.handlers[topic][handlerID] = handler
	k.handlersMutex.Unlock()

	// Создаем конфигурацию потребителя
	kafkaConfig := &kafka.ConfigMap{
		"bootstrap.servers":                  k.brokers,
		"group.id":                           config.GroupID,
		"auto.offset.reset":                  "latest",
		"enable.auto.commit":                 config.AutoCommit,
		"auto.commit.interval.ms":            int(config.AutoCommitInterval.Milliseconds()),
		"max.poll.records":                   config.MaxPollRecords,
		"session.timeout.ms":                 30000,
		"max.poll.interval.ms":               300000,
		"heartbeat.interval.ms":              3000,
		"fetch.min.bytes":                    1,
		"fetch.max.bytes":                    50 * 1024 * 1024, // 50 МБ
		"fetch.wait.max.ms":                  500,
		"queued.min.messages":                100000,
		"queued.max.messages.kbytes":         1024 * 1024, // 1 ГБ
		"statistics.interval.ms":             5000,
		"topic.metadata.refresh.interval.ms": 300000,
		"connections.max.idle.ms":            600000,
		"reconnect.backoff.ms":               50,
		"reconnect.backoff.max.ms":           10000,
	}

	// Создаем потребителя
	consumer, err := kafka.NewConsumer(kafkaConfig)
	if err != nil {
		k.handlersMutex.Lock()
		delete(k.handlers[topic], handlerID)
		k.handlersMutex.Unlock()
		return nil, fmt.Errorf("ошибка создания Kafka consumer: %w", err)
	}

	// Подписываемся на топик
	err = consumer.Subscribe(topic, nil)
	if err != nil {
		k.handlersMutex.Lock()
		delete(k.handlers[topic], handlerID)
		k.handlersMutex.Unlock()
		consumer.Close()
		return nil, fmt.Errorf("ошибка подписки на топик %s: %w", topic, err)
	}

	k.consumersMutex.Lock()
	k.consumers[handlerID] = consumer
	k.consumersMutex.Unlock()

	// обработка сообщений в отдельной горутине
	go k.consumeMessages(ctx, consumer, topic, handlerID, config)

	// функция для отмены подписки
	unsubscribe := func() error {
		k.handlersMutex.Lock()
		delete(k.handlers[topic], handlerID)
		k.handlersMutex.Unlock()

		k.consumersMutex.Lock()
		consumer := k.consumers[handlerID]
		delete(k.consumers, handlerID)
		k.consumersMutex.Unlock()

		if consumer != nil {
			return consumer.Close()
		}
		return nil
	}

	return unsubscribe, nil
}

// SubscribeForTenant подписывается на сообщения для конкретного арендатора
func (k *KafkaMessaging) SubscribeForTenant(ctx context.Context, topic string, handler interfaces.MessageHandler, tenantID string) (func() error, error) {
	// Оборачиваем обработчик, чтобы фильтровать сообщения по арендатору
	tenantHandler := func(ctx context.Context, msg *interfaces.Message) error {
		if msg.TenantID == tenantID {
			return handler(ctx, msg)
		}
		return nil // Игнорируем сообщения для других арендаторов
	}

	return k.Subscribe(ctx, topic, tenantHandler)
}

// SubscribeGroup подписывается на сообщения в составе группы потребителей
func (k *KafkaMessaging) SubscribeGroup(ctx context.Context, topic, groupID string, handler interfaces.MessageHandler) (func() error, error) {
	config := &interfaces.ConsumerConfig{
		GroupID:            groupID,
		AutoCommit:         true,
		AutoCommitInterval: 5 * time.Second,
		MaxPollRecords:     500,
		PollTimeout:        100 * time.Millisecond,
	}
	return k.SubscribeWithConfig(ctx, topic, handler, config)
}

// consumeMessages обрабатывает сообщения из Kafka
func (k *KafkaMessaging) consumeMessages(ctx context.Context, consumer *kafka.Consumer, topic, handlerID string, config *interfaces.ConsumerConfig) {
	for {
		select {
		case <-ctx.Done():
			// Контекст отменен, завершаем обработку
			return
		default:
			// Читаем сообщение с таймаутом
			ev := consumer.Poll(int(config.PollTimeout.Milliseconds()))
			if ev == nil {
				continue
			}

			switch e := ev.(type) {
			case *kafka.Message:
				// Преобразуем сообщение
				msg := kafkaMessageToMessage(e)

				// Получаем обработчик
				k.handlersMutex.RLock()
				handlers, ok := k.handlers[topic]
				if !ok {
					k.handlersMutex.RUnlock()
					continue
				}
				handler, ok := handlers[handlerID]
				k.handlersMutex.RUnlock()
				if !ok {
					continue
				}

				// Обрабатываем сообщение
				err := handler(ctx, msg)
				if err != nil {
					// Логирование ошибки
					// В реальной системе здесь должно быть логирование
					continue
				}

				// Подтверждаем обработку сообщения, если ручной режим
				if !config.AutoCommit {
					if _, err := consumer.CommitMessage(e); err != nil {
						// Логирование ошибки
						// В реальной системе здесь должно быть логирование
					}
				}

			case kafka.Error:
				// Ошибка Kafka
				// В реальной системе здесь должно быть логирование
				if e.Code() == kafka.ErrAllBrokersDown {
					// Критическая ошибка, завершаем обработку
					return
				}

			case kafka.PartitionEOF:
				// Достигнут конец партиции, это нормальная ситуация
				// В реальной системе здесь может быть логирование

			default:
				// Другие события Kafka
				// В реальной системе здесь может быть логирование
			}
		}
	}
}

// Commit подтверждает обработку сообщения
func (k *KafkaMessaging) Commit(ctx context.Context, msg *interfaces.Message) error {
	// В реальной системе здесь должна быть реализация подтверждения для конкретного сообщения
	// Для простоты возвращаем nil
	return nil
}

// CommitBatch подтверждает обработку нескольких сообщений
func (k *KafkaMessaging) CommitBatch(ctx context.Context, msgs []*interfaces.Message) error {
	// В реальной системе здесь должна быть реализация пакетного подтверждения
	// Для простоты возвращаем nil
	return nil
}

// Nack отклоняет сообщение (может привести к повторной обработке)
func (k *KafkaMessaging) Nack(ctx context.Context, msg *interfaces.Message) error {
	// В реальной системе здесь должна быть реализация отклонения сообщения
	// Для простоты возвращаем nil
	return nil
}

// CreateTopic создает новую тему с указанным числом партиций и фактором репликации
func (k *KafkaMessaging) CreateTopic(ctx context.Context, topic string, partitions int, replicationFactor int) error {
	// Создание админ клиента
	adminClient, err := kafka.NewAdminClient(&kafka.ConfigMap{
		"bootstrap.servers": k.brokers,
	})
	if err != nil {
		return fmt.Errorf("ошибка создания Kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// Создание топика
	topicConfig := []kafka.TopicSpecification{
		{
			Topic:             topic,
			NumPartitions:     partitions,
			ReplicationFactor: replicationFactor,
		},
	}

	// Установка таймаута операции
	options := kafka.SetAdminOperationTimeout(30 * time.Second)

	// Выполнение операции создания
	result, err := adminClient.CreateTopics(ctx, topicConfig, options)
	if err != nil {
		return fmt.Errorf("ошибка создания топика %s: %w", topic, err)
	}

	// Проверка результата
	for _, r := range result {
		if r.Error.Code() != kafka.ErrNoError {
			return fmt.Errorf("ошибка создания топика %s: %s", r.Topic, r.Error.String())
		}
	}

	return nil
}

// DeleteTopic удаляет тему
func (k *KafkaMessaging) DeleteTopic(ctx context.Context, topic string) error {
	// Создание админ клиента
	adminClient, err := kafka.NewAdminClient(&kafka.ConfigMap{
		"bootstrap.servers": k.brokers,
	})
	if err != nil {
		return fmt.Errorf("ошибка создания Kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// Установка таймаута операции
	options := kafka.SetAdminOperationTimeout(30 * time.Second)

	// Выполнение операции удаления
	result, err := adminClient.DeleteTopics(ctx, []string{topic}, options)
	if err != nil {
		return fmt.Errorf("ошибка удаления топика %s: %w", topic, err)
	}

	// Проверка результата
	for _, r := range result {
		if r.Error.Code() != kafka.ErrNoError {
			return fmt.Errorf("ошибка удаления топика %s: %s", r.Topic, r.Error.String())
		}
	}

	return nil
}

// ListTopics возвращает список всех тем
func (k *KafkaMessaging) ListTopics(ctx context.Context) ([]string, error) {
	// Создание админ клиента
	adminClient, err := kafka.NewAdminClient(&kafka.ConfigMap{
		"bootstrap.servers": k.brokers,
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// Получение метаданных
	metadata, err := adminClient.GetMetadata(nil, true, 30*1000)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения метаданных Kafka: %w", err)
	}

	// Извлечение списка топиков
	var topics []string
	for topic := range metadata.Topics {
		topics = append(topics, topic)
	}

	return topics, nil
}

// Close закрывает соединение с системой обмена сообщениями
func (k *KafkaMessaging) Close() error {
	// Закрываем все потребители
	k.consumersMutex.Lock()
	for id, consumer := range k.consumers {
		consumer.Close()
		delete(k.consumers, id)
	}
	k.consumersMutex.Unlock()

	// Закрываем producer
	k.producer.Flush(15 * 1000) // Ждем до 15 секунд для отправки всех сообщений
	k.producer.Close()

	return nil
}
