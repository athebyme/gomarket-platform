# Используйте версию Go, соответствующую вашим go.mod файлам
ARG GO_VERSION=1.23.1
ARG ALPINE_VERSION=3.19

# --- Стадия сборки ---
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

RUN apk add --no-cache \
    build-base \
    git \
    pkgconfig \
    librdkafka-dev \
    openssl-dev \
    zlib-dev \
    cyrus-sasl-dev \
    libc6-compat
WORKDIR /app

# 1. Копируем файлы управления зависимостями ВСЕГО воркспейса
COPY go.work .
COPY services/product-service/go.mod services/product-service/go.sum ./services/product-service/
COPY pkg/go.mod ./pkg/

COPY services/marketplace-service/go.mod ./services/marketplace-service/
COPY services/supplier-service/go.mod ./services/supplier-service/

# 2. Скачиваем ВСЕ зависимости воркспейса из корня /app
RUN go mod download

# 3. Теперь копируем исходный код необходимых модулей
COPY services/product-service ./services/product-service/
COPY pkg ./pkg/

# 4. Копируем конфигурацию сервиса
COPY services/product-service/config /app/config/

# 5. Снова переходим в директорию сервиса для сборки
WORKDIR /app/services/product-service

# Создаем директорию для бинарников
RUN mkdir -p /app/bin

# Собираем бинарники
    RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-w -s" \
        -tags musl \
        -o /app/bin/api-server ./cmd/api

    RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-w -s" \
        -tags musl \
        -o /app/bin/worker ./cmd/worker

# --- Финальная стадия ---
FROM alpine:${ALPINE_VERSION}

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    libc6-compat \
    librdkafka \
    wget

ENV TZ=Europe/Moscow

WORKDIR /app

# Копируем артефакты из builder'а
COPY --from=builder /app/bin/api-server /app/api-server
COPY --from=builder /app/bin/worker /app/worker
COPY --from=builder /app/config /app/config
RUN mkdir /app/logs

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN chown -R appuser:appgroup /app

USER appuser

HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
    CMD wget -q --spider http://localhost:${SERVER_PORT}/health || exit 1

ENTRYPOINT ["/bin/sh", "-c"]
CMD ["if [ \"$SERVICE_TYPE\" = \"worker\" ]; then exec /app/worker; else exec /app/api-server; fi"]