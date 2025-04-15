package middleware

import (
	"context"
	"github.com/athebyme/gomarket-platform/pkg/interfaces"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"time"
)

// RequestID добавляет уникальный идентификатор запроса
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), "request_id", requestID)
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logger логирует входящие запросы и время их выполнения
func Logger(logger interfaces.LoggerPort) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Создаем обертку над ResponseWriter для отслеживания статус-кода
			ww := NewResponseWriter(w)

			// Логируем входящий запрос
			requestID, _ := r.Context().Value("request_id").(string)
			logger.InfoWithContext(r.Context(), "Входящий запрос",
				interfaces.LogField{Key: "method", Value: r.Method},
				interfaces.LogField{Key: "path", Value: r.URL.Path},
				interfaces.LogField{Key: "request_id", Value: requestID},
				interfaces.LogField{Key: "remote_addr", Value: r.RemoteAddr},
				interfaces.LogField{Key: "user_agent", Value: r.UserAgent()},
			)

			// Выполняем запрос
			next.ServeHTTP(ww, r)

			// Рассчитываем время выполнения
			duration := time.Since(start)

			// Логируем результат запроса
			logger.InfoWithContext(r.Context(), "Исходящий ответ",
				interfaces.LogField{Key: "method", Value: r.Method},
				interfaces.LogField{Key: "path", Value: r.URL.Path},
				interfaces.LogField{Key: "status", Value: ww.Status()},
				interfaces.LogField{Key: "duration", Value: duration.String()},
				interfaces.LogField{Key: "request_id", Value: requestID},
			)
		})
	}
}

// ResponseWriter обертка для отслеживания статус-кода
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// NewResponseWriter создает новую обертку ResponseWriter
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader записывает статус-код
func (rw *ResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Status возвращает статус-код
func (rw *ResponseWriter) Status() int {
	return rw.statusCode
}

// Recoverer обрабатывает панику в запросах
func Recoverer(logger interfaces.LoggerPort) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					logger.ErrorWithContext(r.Context(), "Паника при обработке запроса",
						interfaces.LogField{Key: "error", Value: rvr},
						interfaces.LogField{Key: "path", Value: r.URL.Path},
						interfaces.LogField{Key: "method", Value: r.Method},
					)

					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// Tenant извлекает ID арендатора из заголовка и добавляет его в контекст
func Tenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			http.Error(w, "X-Tenant-ID header is required", http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), "tenant_id", tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Supplier извлекает ID поставщика из заголовка и добавляет его в контекст
func Supplier(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supplierID := r.Header.Get("X-Supplier-ID")
		if supplierID == "" {
			http.Error(w, "X-Supplier-ID header is required", http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), "supplier_id", supplierID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Auth проверяет аутентификацию по токену
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}

		// Проверяем формат токена
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		token := parts[1]

		// В реальном приложении здесь была бы проверка токена
		// Здесь приведен пример-заглушка
		if token == "" {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Добавляем user_id в контекст (в реальном приложении получали бы из токена)
		ctx := context.WithValue(r.Context(), "user_id", "user123")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Timeout устанавливает таймаут для запроса
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			done := make(chan struct{})

			go func() {
				next.ServeHTTP(w, r.WithContext(ctx))
				close(done)
			}()

			select {
			case <-done:
				return
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					http.Error(w, "Request timeout", http.StatusGatewayTimeout)
				}
				return
			}
		})
	}
}

// CORS добавляет заголовки для Cross-Origin Resource Sharing
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Проверяем, разрешен ли данный origin
			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || origin == allowedOrigin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization, X-Tenant-ID, X-Request-ID")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Обрабатываем предварительные запросы OPTIONS
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Tracing добавляет трассировку запросов
func Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// В реальном приложении здесь был бы код для трассировки запросов
		// Например, с использованием OpenTelemetry или Jaeger

		// Получаем или генерируем trace_id
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), "trace_id", traceID)
		w.Header().Set("X-Trace-ID", traceID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RateLimit ограничивает количество запросов
func RateLimit(requestsPerSecond int) func(http.Handler) http.Handler {
	// В реальном приложении здесь был бы код для ограничения количества запросов
	// Например, с использованием алгоритма "token bucket"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Здесь приведена заглушка
			next.ServeHTTP(w, r)
		})
	}
}
