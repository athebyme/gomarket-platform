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

type KafkaMessaging struct {
	producer       *kafka.Producer
	consumers      map[string]*kafka.Consumer
	consumersMutex sync.RWMutex
	handlers       map[string]interfaces.MessageHandler
	handlersMutex  sync.RWMutex
	brokers        []string
	groupID        string
}

func NewKafkaMessaging(brokers []string, groupID string) (interfaces.MessagingPort, error) {
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"client.id":         "product-service-producer",
		"acks":              "all",    // максимальная надежность
		"retries":           5,        // количество повторных попыток
		"retry.backoff.ms":  500,      // задержка между повторами
		"compression.type":  "snappy", // алгоритм сжатия для экономии трафика
		"linger.ms":         10,       // задержка для батчинга сообщений
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Kafka producer: %w", err)
	}

	return &KafkaMessaging{
		producer:       producer,
		consumers:      make(map[string]*kafka.Consumer),
		consumersMutex: sync.RWMutex{},
		handlers:       make(map[string]interfaces.MessageHandler),
		handlersMutex:  sync.RWMutex{},
		brokers:        brokers,
		groupID:        groupID,
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

	return k.producer.Produce(msg, nil)
}

func (k *KafkaMessaging) Subscribe(ctx context.Context, topic string, handler interfaces.MessageHandler) (func() error, error) {
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":       k.brokers,
		"group.id":                k.groupID,
		"auto.offset.reset":       "latest",
		"enable.auto.commit":      true,
		"auto.commit.interval.ms": 5000,
		"session.timeout.ms":      30000,
		"max.poll.interval.ms":    300000,
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Kafka consumer: %w", err)
	}

	err = consumer.Subscribe(topic, nil)
	if err != nil {
		consumer.Close()
		return nil, fmt.Errorf("ошибка подписки на топик %s: %w", topic, err)
	}

	consumerID := uuid.New().String()
	k.consumersMutex.Lock()
	k.consumers[consumerID] = consumer
	k.consumersMutex.Unlock()

	k.handlersMutex.Lock()
	k.handlers[consumerID] = handler
	k.handlersMutex.Unlock()

	go k.consumeMessages(ctx, consumer, consumerID)

	unsubscribe := func() error {
		k.consumersMutex.Lock()
		defer k.consumersMutex.Unlock()

		k.handlersMutex.Lock()
		delete(k.handlers, consumerID)
		k.handlersMutex.Unlock()

		if c, exists := k.consumers[consumerID]; exists {
			delete(k.consumers, consumerID)
			return c.Close()
		}
		return nil
	}

	return unsubscribe, nil
}

func (k *KafkaMessaging) consumeMessages(ctx context.Context, consumer *kafka.Consumer, consumerID string) {
	const maxRetries = 3

	for {
		select {
		case <-ctx.Done():
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
					continue
				}

				msg := k.kafkaToInterfaceMessage(e)

				var err error
				for attempt := 0; attempt < maxRetries; attempt++ {
					msg.Attempts++
					err = handler(ctx, msg)
					if err == nil {
						break
					}

					backoff := time.Duration(attempt+1) * 500 * time.Millisecond
					time.Sleep(backoff)
				}

			case kafka.Error:
				if e.Code() == kafka.ErrAllBrokersDown {
					return
				}
			}
		}
	}
}

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
		TenantID:    headers["tenant_id"],
		PublishedAt: time.Now(),
		Attempts:    0,
	}
}

func (k *KafkaMessaging) Close() error {
	k.consumersMutex.Lock()
	for id, consumer := range k.consumers {
		consumer.Close()
		delete(k.consumers, id)
	}
	k.consumersMutex.Unlock()

	k.producer.Flush(5000)
	k.producer.Close()

	return nil
}
