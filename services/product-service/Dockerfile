FROM golang:1.19-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /app/bin/api-server ./cmd/api

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /app/bin/worker ./cmd/worker

FROM alpine:3.17

RUN apk add --no-cache ca-certificates tzdata libc6-compat

ENV TZ=Europe/Moscow

COPY --from=builder /app/config /app/config

RUN mkdir -p /app/logs

COPY --from=builder /app/bin/api-server /app/api-server
COPY --from=builder /app/bin/worker /app/worker

ENV APP_ENV=production
ENV LOG_LEVEL=info
ENV SERVER_PORT=8080

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN chown -R appuser:appgroup /app

USER appuser

WORKDIR /app

ENTRYPOINT ["/bin/sh", "-c"]
CMD ["if [ \"$SERVICE_TYPE\" = \"worker\" ]; then exec /app/worker; else exec /app/api-server; fi"]