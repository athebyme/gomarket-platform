FROM golang:1.19-alpine AS builder

# Install dependencies
RUN apk add --no-cache git gcc g++ make musl-dev pkgconfig libc-dev
RUN apk add --no-cache librdkafka-dev libc6-compat

WORKDIR /app

# Copy the entire project
COPY . .

# Create a replacement directive in the product-service go.mod
WORKDIR /app/services/product-service
RUN echo "replace github.com/athebyme/gomarket-platform => ../../" >> go.mod

# Now run go mod tidy which should use the local module
RUN go mod tidy

# Build the binaries
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /app/bin/api-server ./cmd/api
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /app/bin/worker ./cmd/worker

# Финальный образ
FROM alpine:3.17

# Установка необходимых зависимостей для запуска
RUN apk add --no-cache ca-certificates tzdata libc6-compat librdkafka

ENV TZ=Europe/Moscow

# Копирование конфигурации и бинарных файлов
COPY --from=builder /app/config /app/config
RUN mkdir -p /app/logs
COPY --from=builder /app/bin/api-server /app/api-server
COPY --from=builder /app/bin/worker /app/worker

# Настройка окружения
ENV APP_ENV=production
ENV LOG_LEVEL=info
ENV SERVER_PORT=8080

EXPOSE 8080

# Проверка работоспособности
HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

# Создание непривилегированного пользователя
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN chown -R appuser:appgroup /app

USER appuser

WORKDIR /app

# Запуск сервиса в зависимости от переменной окружения
ENTRYPOINT ["/bin/sh", "-c"]
CMD ["if [ \"$SERVICE_TYPE\" = \"worker\" ]; then exec /app/worker; else exec /app/api-server; fi"]