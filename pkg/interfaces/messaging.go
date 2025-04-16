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

type MessagingPort interface {
	Publish(ctx context.Context, topic string, message []byte) error

	Subscribe(ctx context.Context, topic string, handler MessageHandler) (func() error, error)

	Close() error
}
