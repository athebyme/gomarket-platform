package interfaces

import (
	"context"
	"time"
)

// Message представляет сообщение в системе
type Message struct {
	ID          string                 `json:"id"`           // Уникальный ID сообщения
	Topic       string                 `json:"topic"`        // Тема сообщения
	Key         string                 `json:"key"`          // Ключ сообщения (опционально)
	Value       []byte                 `json:"value"`        // Содержимое сообщения
	Headers     map[string]string      `json:"headers"`      // Заголовки сообщения
	Metadata    map[string]interface{} `json:"metadata"`     // Метаданные сообщения
	TenantID    string                 `json:"tenant_id"`    // ID арендатора (для многоарендности)
	PublishedAt time.Time              `json:"published_at"` // Время публикации
	Attempts    int                    `json:"attempts"`     // Число попыток доставки
}

// MessageHandler определяет функцию обработчика сообщений
type MessageHandler func(ctx context.Context, msg *Message) error

// ConsumerConfig содержит настройки для подписчика на сообщения
type ConsumerConfig struct {
	GroupID            string        // ID группы потребителей
	AutoCommit         bool          // Автоматически подтверждать полученные сообщения
	AutoCommitInterval time.Duration // Интервал автоматического подтверждения
	MaxPollRecords     int           // Максимальное число сообщений за один запрос
	PollTimeout        time.Duration // Таймаут для опроса новых сообщений
	TenantID           string        // ID арендатора (для многоарендности)
}

// ProducerConfig содержит настройки для отправителя сообщений
type ProducerConfig struct {
	Async        bool          // Асинхронная отправка сообщений
	BatchSize    int           // Размер пакета сообщений
	LingerMs     time.Duration // Время ожидания заполнения пакета
	Compression  string        // Тип сжатия (none, gzip, snappy, lz4, zstd)
	RequiredAcks int           // Требуемое число подтверждений (-1 - от всех реплик)
	RetryBackoff time.Duration // Время между повторными попытками
	MaxRetries   int           // Максимальное число повторных попыток
	TenantID     string        // ID арендатора (для многоарендности)
}

// MessagingPort определяет интерфейс для работы с системой обмена сообщениями
// Реализация может использовать Kafka, RabbitMQ, NATS или другие системы
type MessagingPort interface {
	// Публикация сообщений

	// Publish публикует сообщение в указанную тему
	Publish(ctx context.Context, topic string, message []byte) error

	// PublishWithKey публикует сообщение с указанным ключом
	// Ключ может использоваться для партиционирования или маршрутизации
	PublishWithKey(ctx context.Context, topic string, key string, message []byte) error

	// PublishWithHeaders публикует сообщение с дополнительными заголовками
	PublishWithHeaders(ctx context.Context, topic string, message []byte, headers map[string]string) error

	// PublishForTenant публикует сообщение с учетом ID арендатора
	PublishForTenant(ctx context.Context, topic string, message []byte, tenantID string) error

	// PublishBatch публикует несколько сообщений одним запросом
	PublishBatch(ctx context.Context, topic string, messages [][]byte) error

	// PublishBatchWithKeys публикует несколько сообщений с ключами одним запросом
	PublishBatchWithKeys(ctx context.Context, topic string, keyedMessages map[string][]byte) error

	// Подписка на сообщения

	// Subscribe подписывается на указанную тему и обрабатывает сообщения с помощью handler
	// Возвращает функцию для отмены подписки
	Subscribe(ctx context.Context, topic string, handler MessageHandler) (func() error, error)

	// SubscribeWithConfig подписывается на указанную тему с дополнительными настройками
	SubscribeWithConfig(ctx context.Context, topic string, handler MessageHandler, config *ConsumerConfig) (func() error, error)

	// SubscribeForTenant подписывается на сообщения для конкретного арендатора
	SubscribeForTenant(ctx context.Context, topic string, handler MessageHandler, tenantID string) (func() error, error)

	// SubscribeGroup подписывается на сообщения в составе группы потребителей
	// Сообщения распределяются между подписчиками в группе
	SubscribeGroup(ctx context.Context, topic, groupID string, handler MessageHandler) (func() error, error)

	// Управление подтверждениями

	// Commit подтверждает обработку сообщения
	// Имеет смысл только при ручном подтверждении (AutoCommit = false)
	Commit(ctx context.Context, msg *Message) error

	// CommitBatch подтверждает обработку нескольких сообщений
	CommitBatch(ctx context.Context, msgs []*Message) error

	// Nack отклоняет сообщение (может привести к повторной обработке)
	Nack(ctx context.Context, msg *Message) error

	// Управление топиками

	// CreateTopic создает новую тему с указанным числом партиций и фактором репликации
	CreateTopic(ctx context.Context, topic string, partitions int, replicationFactor int) error

	// DeleteTopic удаляет тему
	DeleteTopic(ctx context.Context, topic string) error

	// ListTopics возвращает список всех тем
	ListTopics(ctx context.Context) ([]string, error)

	// Управление соединением

	// Close закрывает соединение с системой обмена сообщениями
	Close() error
}
