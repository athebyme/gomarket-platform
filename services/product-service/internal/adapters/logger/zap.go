package logger

import (
	"context"
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"sync"
)

var (
	instance *ZapLogger
	once     sync.Once
)

// ZapLogger адаптер для Zap, реализующий LoggerPort
type ZapLogger struct {
	logger *zap.SugaredLogger
}

// NewZapLogger создает новый логгер на основе Zap
func NewZapLogger(level string, isProduction bool) (interfaces.LoggerPort, error) {
	var err error
	once.Do(func() {
		instance = &ZapLogger{}
		err = instance.init(level, isProduction)
	})

	if err != nil {
		return nil, err
	}

	return instance, nil
}

// init инициализирует логгер
func (z *ZapLogger) init(levelStr string, isProduction bool) error {
	var config zap.Config

	if isProduction {
		config = zap.NewProductionConfig()
		// Настройки для production
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentConfig()
		// Настройки для development
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.DisableCaller = false
		config.DisableStacktrace = false
	}

	// Парсинг уровня логирования
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		level = zapcore.InfoLevel
	}
	config.Level = zap.NewAtomicLevelAt(level)

	// Настройка вывода
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	// Создание логгера
	logger, err := config.Build()
	if err != nil {
		return err
	}

	z.logger = logger.Sugar()
	return nil
}

// GetLoggerLevel преобразует строковый уровень логирования в LogLevel
func GetLoggerLevel(levelStr string) interfaces.LogLevel {
	switch levelStr {
	case "debug":
		return interfaces.DebugLevel
	case "info":
		return interfaces.InfoLevel
	case "warn":
		return interfaces.WarnLevel
	case "error":
		return interfaces.ErrorLevel
	case "fatal":
		return interfaces.FatalLevel
	case "panic":
		return interfaces.PanicLevel
	default:
		return interfaces.InfoLevel
	}
}

// convertToZapFields преобразует LogField в zap.Field
func convertToZapFields(args ...interface{}) []interface{} {
	for i, arg := range args {
		if field, ok := arg.(interfaces.LogField); ok {
			args[i] = zap.Any(field.Key, field.Value)
		}
	}
	return args
}

// extractFieldsFromContext извлекает поля из контекста
func (z *ZapLogger) extractFieldsFromContext(ctx context.Context) []interface{} {
	// Если бы в контексте хранились дополнительные поля, их можно было бы извлечь здесь
	// Например, traceID, requestID и т.д.
	var fields []interface{}

	// Пример: добавление request_id, если оно есть в контексте
	if reqID, ok := ctx.Value("request_id").(string); ok {
		fields = append(fields, zap.String("request_id", reqID))
	}

	// Пример: добавление tenant_id, если оно есть в контексте
	if tenantID, ok := ctx.Value("tenant_id").(string); ok {
		fields = append(fields, zap.String("tenant_id", tenantID))
	}

	// Пример: добавление user_id, если оно есть в контексте
	if userID, ok := ctx.Value("user_id").(string); ok {
		fields = append(fields, zap.String("user_id", userID))
	}

	return fields
}

// Debug реализация интерфейса LoggerPort
func (z *ZapLogger) Debug(msg string, args ...interface{}) {
	z.logger.Debugw(msg, convertToZapFields(args...)...)
}

// Info реализация интерфейса LoggerPort
func (z *ZapLogger) Info(msg string, args ...interface{}) {
	z.logger.Infow(msg, convertToZapFields(args...)...)
}

// Warn реализация интерфейса LoggerPort
func (z *ZapLogger) Warn(msg string, args ...interface{}) {
	z.logger.Warnw(msg, convertToZapFields(args...)...)
}

// Error реализация интерфейса LoggerPort
func (z *ZapLogger) Error(msg string, args ...interface{}) {
	z.logger.Errorw(msg, convertToZapFields(args...)...)
}

// Fatal реализация интерфейса LoggerPort
func (z *ZapLogger) Fatal(msg string, args ...interface{}) {
	z.logger.Fatalw(msg, convertToZapFields(args...)...)
	os.Exit(1)
}

// Panic реализация интерфейса LoggerPort
func (z *ZapLogger) Panic(msg string, args ...interface{}) {
	z.logger.Panicw(msg, convertToZapFields(args...)...)
}

// DebugWithContext реализация интерфейса LoggerPort
func (z *ZapLogger) DebugWithContext(ctx context.Context, msg string, args ...interface{}) {
	fields := z.extractFieldsFromContext(ctx)
	z.logger.Debugw(msg, append(convertToZapFields(args...), fields...)...)
}

// InfoWithContext реализация интерфейса LoggerPort
func (z *ZapLogger) InfoWithContext(ctx context.Context, msg string, args ...interface{}) {
	fields := z.extractFieldsFromContext(ctx)
	z.logger.Infow(msg, append(convertToZapFields(args...), fields...)...)
}

// WarnWithContext реализация интерфейса LoggerPort
func (z *ZapLogger) WarnWithContext(ctx context.Context, msg string, args ...interface{}) {
	fields := z.extractFieldsFromContext(ctx)
	z.logger.Warnw(msg, append(convertToZapFields(args...), fields...)...)
}

// ErrorWithContext реализация интерфейса LoggerPort
func (z *ZapLogger) ErrorWithContext(ctx context.Context, msg string, args ...interface{}) {
	fields := z.extractFieldsFromContext(ctx)
	z.logger.Errorw(msg, append(convertToZapFields(args...), fields...)...)
}

// FatalWithContext реализация интерфейса LoggerPort
func (z *ZapLogger) FatalWithContext(ctx context.Context, msg string, args ...interface{}) {
	fields := z.extractFieldsFromContext(ctx)
	z.logger.Fatalw(msg, append(convertToZapFields(args...), fields...)...)
	os.Exit(1)
}

// PanicWithContext реализация интерфейса LoggerPort
func (z *ZapLogger) PanicWithContext(ctx context.Context, msg string, args ...interface{}) {
	fields := z.extractFieldsFromContext(ctx)
	z.logger.Panicw(msg, append(convertToZapFields(args...), fields...)...)
}

// WithFields реализация интерфейса LoggerPort
func (z *ZapLogger) WithFields(fields ...interfaces.LogField) interfaces.LoggerPort {
	newLogger := &ZapLogger{}
	zapFields := make([]interface{}, 0, len(fields)*2)
	for _, field := range fields {
		zapFields = append(zapFields, field.Key, field.Value)
	}
	newLogger.logger = z.logger.With(zapFields...)
	return newLogger
}

// WithField реализация интерфейса LoggerPort
func (z *ZapLogger) WithField(key string, value interface{}) interfaces.LoggerPort {
	newLogger := &ZapLogger{}
	newLogger.logger = z.logger.With(key, value)
	return newLogger
}

// WithTenant реализация интерфейса LoggerPort
func (z *ZapLogger) WithTenant(tenantID string) interfaces.LoggerPort {
	return z.WithField("tenant_id", tenantID)
}

// WithTraceID реализация интерфейса LoggerPort
func (z *ZapLogger) WithTraceID(traceID string) interfaces.LoggerPort {
	return z.WithField("trace_id", traceID)
}

// SetLevel реализация интерфейса LoggerPort
func (z *ZapLogger) SetLevel(level interfaces.LogLevel) {
	var _ zapcore.Level
	switch level {
	case interfaces.DebugLevel:
		_ = zapcore.DebugLevel
	case interfaces.InfoLevel:
		_ = zapcore.InfoLevel
	case interfaces.WarnLevel:
		_ = zapcore.WarnLevel
	case interfaces.ErrorLevel:
		_ = zapcore.ErrorLevel
	case interfaces.FatalLevel:
		_ = zapcore.FatalLevel
	case interfaces.PanicLevel:
		_ = zapcore.PanicLevel
	default:
		_ = zapcore.InfoLevel
	}

	// Предполагается, что у logger есть встроенный level
	// Это упрощение, в реальном коде необходимо получить атомический уровень из логгера и установить его
}

// GetLevel реализация интерфейса LoggerPort
func (z *ZapLogger) GetLevel() interfaces.LogLevel {
	// Упрощение, в реальном коде необходимо получить атомический уровень из логгера
	return interfaces.InfoLevel
}

// Flush реализация интерфейса LoggerPort
func (z *ZapLogger) Flush() error {
	return z.logger.Sync()
}

// Sync реализация интерфейса LoggerPort
func (z *ZapLogger) Sync() error {
	return z.logger.Sync()
}
