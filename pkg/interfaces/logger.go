package interfaces

import "context"

// LogLevel определяет уровни логирования
type LogLevel int

const (
	// Уровни логирования от наименее до наиболее важного
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	PanicLevel
)

// LogField представляет дополнительное поле в логе
type LogField struct {
	Key   string
	Value interface{}
}

// LoggerPort определяет интерфейс для системы логирования
// Реализация может использовать любую библиотеку логирования (Zap, Logrus, Zerolog и т.д.)
type LoggerPort interface {
	// Методы логирования с разными уровнями

	// Debug логирует сообщение с уровнем Debug
	Debug(msg string, args ...interface{})

	// Info логирует сообщение с уровнем Info
	Info(msg string, args ...interface{})

	// Warn логирует сообщение с уровнем Warn
	Warn(msg string, args ...interface{})

	// Error логирует сообщение с уровнем Error
	Error(msg string, args ...interface{})

	// Fatal логирует сообщение с уровнем Fatal и завершает программу
	Fatal(msg string, args ...interface{})

	// Panic логирует сообщение с уровнем Panic и вызывает панику
	Panic(msg string, args ...interface{})

	// Методы логирования с контекстом

	// DebugWithContext логирует сообщение с контекстом
	DebugWithContext(ctx context.Context, msg string, args ...interface{})

	// InfoWithContext логирует сообщение с контекстом
	InfoWithContext(ctx context.Context, msg string, args ...interface{})

	// WarnWithContext логирует сообщение с контекстом
	WarnWithContext(ctx context.Context, msg string, args ...interface{})

	// ErrorWithContext логирует сообщение с контекстом
	ErrorWithContext(ctx context.Context, msg string, args ...interface{})

	// FatalWithContext логирует сообщение с контекстом и завершает программу
	FatalWithContext(ctx context.Context, msg string, args ...interface{})

	// PanicWithContext логирует сообщение с контекстом и вызывает панику
	PanicWithContext(ctx context.Context, msg string, args ...interface{})

	// Методы для работы с полями

	// WithFields возвращает новый логгер с добавленными полями
	WithFields(fields ...LogField) LoggerPort

	// WithField возвращает новый логгер с добавленным полем
	WithField(key string, value interface{}) LoggerPort

	// Методы для многоарендности

	// WithTenant возвращает новый логгер с добавленным идентификатором арендатора
	WithTenant(tenantID string) LoggerPort

	// Методы для трассировки

	// WithTraceID возвращает новый логгер с добавленным идентификатором трассировки
	WithTraceID(traceID string) LoggerPort

	// Методы для настройки

	// SetLevel устанавливает минимальный уровень логирования
	SetLevel(level LogLevel)

	// GetLevel возвращает текущий уровень логирования
	GetLevel() LogLevel

	// Flush сбрасывает буферы и гарантирует запись всех сообщений
	Flush() error

	// Sync синхронизирует записи буфера с хранилищем логов
	Sync() error
}
