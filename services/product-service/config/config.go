package config

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"strings"
	"time"
)

// Config содержит все настройки сервиса
type Config struct {
	AppName  string
	Version  string
	LogLevel string
	ENV      string

	Server struct {
		Host            string
		Port            int
		ReadTimeout     time.Duration
		WriteTimeout    time.Duration
		ShutdownTimeout time.Duration
		BodyLimit       int // максимальный размер запроса в МБ
	}

	Postgres struct {
		Host     string
		Port     int
		User     string
		Password string
		DBName   string
		SSLMode  string
		Timeout  time.Duration
		PoolSize int // размер пула соединений
	}

	Redis struct {
		Host              string
		Port              int
		Password          string
		DB                int
		PoolSize          int           // размер пула соединений
		MinIdleConns      int           // минимальное количество неактивных соединений
		ConnectTimeout    time.Duration // таймаут соединения
		ReadTimeout       time.Duration // таймаут чтения
		WriteTimeout      time.Duration // таймаут записи
		PoolTimeout       time.Duration // таймаут ожидания соединения из пула
		IdleTimeout       time.Duration // таймаут неактивного соединения
		IdleCheckFreq     time.Duration // частота проверки неактивных соединений
		MaxRetries        int           // максимальное количество повторных попыток
		MinRetryBackoff   time.Duration // минимальное время между повторными попытками
		MaxRetryBackoff   time.Duration // максимальное время между повторными попытками
		DefaultExpiration time.Duration // срок действия кэша по умолчанию
	}

	Kafka struct {
		Brokers           []string      `mapstructure:"brokers"`
		GroupID           string        `mapstructure:"group_id"`
		ProducerTopic     string        `mapstructure:"producer_topic"`
		ConsumerTopic     string        `mapstructure:"consumer_topic"`
		DeadLetterTopic   string        `mapstructure:"dead_letter_topic"`
		AutoOffsetReset   string        `mapstructure:"auto_offset_reset"`
		SessionTimeout    time.Duration `mapstructure:"session_timeout"`
		HeartbeatTimeout  time.Duration `mapstructure:"heartbeat_timeout"`
		ReadTimeout       time.Duration `mapstructure:"read_timeout"`
		WriteTimeout      time.Duration `mapstructure:"write_timeout"`
		MaxRetries        int           `mapstructure:"max_retries"`
		RetryBackoff      time.Duration `mapstructure:"retry_backoff"`
		BatchSize         int           `mapstructure:"batch_size"`
		LingerMs          int           `mapstructure:"linger_ms"`
		EnableIdempotence bool          `mapstructure:"enable_idempotence"`
		CompressionType   string        `mapstructure:"compression_type"`
	}

	Tracing struct {
		Enabled     bool
		ServiceName string
		Endpoint    string
		Probability float64 // вероятность сэмплирования трассировки
	}

	Metrics struct {
		Enabled     bool
		ServiceName string
		Endpoint    string
		Port        int `mapstructure:"port"`
	}

	Security struct {
		JWTSecret        string
		JWTExpirationMin time.Duration
		CORSAllowOrigins []string
	}

	Resilience struct {
		MaxRetries      int           // максимальное число повторов
		RetryWaitTime   time.Duration // время ожидания между повторами
		CircuitTimeout  time.Duration // таймаут для размыкания цепи
		HalfOpenMaxReqs int           // макс. запросов в полуоткрытом состоянии
		TripThreshold   int           // порог ошибок для размыкания
	}
}

// Load загружает конфигурацию из файла и переменных окружения
func Load(configPath string) (*Config, error) {
	configFile := "config"
	if configPath != "" {
		configFile = configPath
	}

	var cfg Config

	// Настройка Viper
	viper.SetConfigName(configFile)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("../config")
	viper.AddConfigPath("../../config")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Чтение конфигурационного файла
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("ошибка чтения файла конфигурации: %w", err)
		}
		// Продолжаем, если файл не найден, будем использовать только переменные окружения
	}

	// Установка значений по умолчанию
	setDefaults()

	// Привязка переменных окружения
	bindEnvVariables()

	// Чтение конфигурации в структуру
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("ошибка десериализации конфигурации: %w", err)
	}

	// Получаем окружение
	cfg.ENV = viper.GetString("env")
	if cfg.ENV == "" {
		cfg.ENV = "development"
		if envVar := os.Getenv("APP_ENV"); envVar != "" {
			cfg.ENV = envVar
		}
	}

	return &cfg, nil
}

// setDefaults устанавливает значения по умолчанию
func setDefaults() {
	// Основные настройки
	viper.SetDefault("appName", "product-service")
	viper.SetDefault("version", "1.0.0")
	viper.SetDefault("logLevel", "info")
	viper.SetDefault("env", "development")

	// Настройки сервера
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.readTimeout", "10s")
	viper.SetDefault("server.writeTimeout", "10s")
	viper.SetDefault("server.shutdownTimeout", "5s")
	viper.SetDefault("server.bodyLimit", 10) // 10 МБ

	// Настройки Postgres
	viper.SetDefault("postgres.host", "localhost")
	viper.SetDefault("postgres.port", 5432)
	viper.SetDefault("postgres.user", "postgres")
	viper.SetDefault("postgres.password", "postgres")
	viper.SetDefault("postgres.dbname", "postgres")
	viper.SetDefault("postgres.sslmode", "disable")
	viper.SetDefault("postgres.timeout", "5s")
	viper.SetDefault("postgres.poolSize", 10)

	// Настройки Redis
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.poolSize", 10)
	viper.SetDefault("redis.minIdleConns", 2)
	viper.SetDefault("redis.connectTimeout", "1s")
	viper.SetDefault("redis.readTimeout", "1s")
	viper.SetDefault("redis.writeTimeout", "1s")
	viper.SetDefault("redis.poolTimeout", "4s")
	viper.SetDefault("redis.idleTimeout", "300s")
	viper.SetDefault("redis.idleCheckFreq", "60s")
	viper.SetDefault("redis.maxRetries", 3)
	viper.SetDefault("redis.minRetryBackoff", "8ms")
	viper.SetDefault("redis.maxRetryBackoff", "512ms")
	viper.SetDefault("redis.defaultExpiration", "10m")

	// Настройки Kafka
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.groupID", "product-service")
	viper.SetDefault("kafka.topic", "products")
	viper.SetDefault("kafka.producerTopic", "products-producer")
	viper.SetDefault("kafka.consumerTopic", "products-consumer")
	viper.SetDefault("kafka.autoOffsetReset", "latest")
	viper.SetDefault("kafka.sessionTimeout", "10s")
	viper.SetDefault("kafka.heartbeatTimeout", "3s")
	viper.SetDefault("kafka.readTimeout", "10s")
	viper.SetDefault("kafka.writeTimeout", "10s")

	// Настройки трассировки
	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.serviceName", "product-service")
	viper.SetDefault("tracing.endpoint", "jaeger:6831")
	viper.SetDefault("tracing.probability", 0.1)

	// Настройки метрик
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.serviceName", "product-service")
	viper.SetDefault("metrics.endpoint", "/metrics")

	// Настройки безопасности
	viper.SetDefault("security.jwtSecret", "your-secret-key")
	viper.SetDefault("security.jwtExpirationMin", "60m")
	viper.SetDefault("security.corsAllowOrigins", []string{"*"})

	// Настройки отказоустойчивости
	viper.SetDefault("resilience.maxRetries", 3)
	viper.SetDefault("resilience.retryWaitTime", "100ms")
	viper.SetDefault("resilience.circuitTimeout", "30s")
	viper.SetDefault("resilience.halfOpenMaxReqs", 5)
	viper.SetDefault("resilience.tripThreshold", 10)
}

// bindEnvVariables привязывает переменные окружения к конфигурации
func bindEnvVariables() {
	// Основные настройки
	viper.BindEnv("appName", "APP_NAME")
	viper.BindEnv("version", "APP_VERSION")
	viper.BindEnv("logLevel", "LOG_LEVEL")
	viper.BindEnv("env", "APP_ENV")

	// Настройки сервера
	viper.BindEnv("server.host", "SERVER_HOST")
	viper.BindEnv("server.port", "SERVER_PORT")
	viper.BindEnv("server.readTimeout", "SERVER_READ_TIMEOUT")
	viper.BindEnv("server.writeTimeout", "SERVER_WRITE_TIMEOUT")
	viper.BindEnv("server.shutdownTimeout", "SERVER_SHUTDOWN_TIMEOUT")
	viper.BindEnv("server.bodyLimit", "SERVER_BODY_LIMIT")

	// Настройки Postgres
	viper.BindEnv("postgres.host", "POSTGRES_HOST")
	viper.BindEnv("postgres.port", "POSTGRES_PORT")
	viper.BindEnv("postgres.user", "POSTGRES_USER")
	viper.BindEnv("postgres.password", "POSTGRES_PASSWORD")
	viper.BindEnv("postgres.dbname", "POSTGRES_DBNAME")
	viper.BindEnv("postgres.sslmode", "POSTGRES_SSLMODE")
	viper.BindEnv("postgres.timeout", "POSTGRES_TIMEOUT")
	viper.BindEnv("postgres.poolSize", "POSTGRES_POOL_SIZE")

	// Настройки Redis
	viper.BindEnv("redis.host", "REDIS_HOST")
	viper.BindEnv("redis.port", "REDIS_PORT")
	viper.BindEnv("redis.password", "REDIS_PASSWORD")
	viper.BindEnv("redis.db", "REDIS_DB")
	viper.BindEnv("redis.poolSize", "REDIS_POOL_SIZE")
	viper.BindEnv("redis.minIdleConns", "REDIS_MIN_IDLE_CONNS")
	viper.BindEnv("redis.connectTimeout", "REDIS_CONNECT_TIMEOUT")
	viper.BindEnv("redis.readTimeout", "REDIS_READ_TIMEOUT")
	viper.BindEnv("redis.writeTimeout", "REDIS_WRITE_TIMEOUT")
	viper.BindEnv("redis.poolTimeout", "REDIS_POOL_TIMEOUT")
	viper.BindEnv("redis.idleTimeout", "REDIS_IDLE_TIMEOUT")
	viper.BindEnv("redis.idleCheckFreq", "REDIS_IDLE_CHECK_FREQ")
	viper.BindEnv("redis.maxRetries", "REDIS_MAX_RETRIES")
	viper.BindEnv("redis.minRetryBackoff", "REDIS_MIN_RETRY_BACKOFF")
	viper.BindEnv("redis.maxRetryBackoff", "REDIS_MAX_RETRY_BACKOFF")
	viper.BindEnv("redis.defaultExpiration", "REDIS_DEFAULT_EXPIRATION")

	// Настройки Kafka
	viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	viper.BindEnv("kafka.groupID", "KAFKA_GROUP_ID")
	viper.BindEnv("kafka.topic", "KAFKA_TOPIC")
	viper.BindEnv("kafka.producerTopic", "KAFKA_PRODUCER_TOPIC")
	viper.BindEnv("kafka.consumerTopic", "KAFKA_CONSUMER_TOPIC")
	viper.BindEnv("kafka.autoOffsetReset", "KAFKA_AUTO_OFFSET_RESET")
	viper.BindEnv("kafka.sessionTimeout", "KAFKA_SESSION_TIMEOUT")
	viper.BindEnv("kafka.heartbeatTimeout", "KAFKA_HEARTBEAT_TIMEOUT")
	viper.BindEnv("kafka.readTimeout", "KAFKA_READ_TIMEOUT")
	viper.BindEnv("kafka.writeTimeout", "KAFKA_WRITE_TIMEOUT")

	// Настройки трассировки
	viper.BindEnv("tracing.enabled", "TRACING_ENABLED")
	viper.BindEnv("tracing.serviceName", "TRACING_SERVICE_NAME")
	viper.BindEnv("tracing.endpoint", "TRACING_ENDPOINT")
	viper.BindEnv("tracing.probability", "TRACING_PROBABILITY")

	// Настройки метрик
	viper.BindEnv("metrics.enabled", "METRICS_ENABLED")
	viper.BindEnv("metrics.serviceName", "METRICS_SERVICE_NAME")
	viper.BindEnv("metrics.endpoint", "METRICS_ENDPOINT")

	// Настройки безопасности
	viper.BindEnv("security.jwtSecret", "JWT_SECRET")
	viper.BindEnv("security.jwtExpirationMin", "JWT_EXPIRATION_MIN")
	viper.BindEnv("security.corsAllowOrigins", "CORS_ALLOW_ORIGINS")

	// Настройки отказоустойчивости
	viper.BindEnv("resilience.maxRetries", "RESILIENCE_MAX_RETRIES")
	viper.BindEnv("resilience.retryWaitTime", "RESILIENCE_RETRY_WAIT_TIME")
	viper.BindEnv("resilience.circuitTimeout", "RESILIENCE_CIRCUIT_TIMEOUT")
	viper.BindEnv("resilience.halfOpenMaxReqs", "RESILIENCE_HALF_OPEN_MAX_REQS")
	viper.BindEnv("resilience.tripThreshold", "RESILIENCE_TRIP_THRESHOLD")
}
