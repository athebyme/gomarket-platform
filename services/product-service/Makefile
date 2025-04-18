.PHONY: build clean run docker-build docker-run docker-stop up down logs help

# Параметры проекта
APP_NAME=product-service
GO_SRC=./cmd/api/main.go ./cmd/worker/main.go
API_SERVER_BIN=./bin/api-server
WORKER_BIN=./bin/worker
BUILD_FLAGS=-ldflags="-s -w"

# Docker параметры
DOCKER_IMAGE_NAME=product-service
DOCKER_COMPOSE_FILE=docker-compose.yml

help:
	@echo "Команды для управления сервисом продуктов:"
	@echo "  make build           - Собрать API-сервер и worker"
	@echo "  make clean           - Удалить бинарные файлы и временные файлы"
	@echo "  make run-api         - Запустить API-сервер локально"
	@echo "  make run-worker      - Запустить worker локально"
	@echo "  make docker-build    - Собрать Docker-образ"
	@echo "  make up              - Запустить все сервисы с docker-compose"
	@echo "  make up-deps         - Запустить только зависимости (PostgreSQL, Redis, Kafka, и т.д.)"
	@echo "  make up-service      - Запустить только API-сервер и worker"
	@echo "  make down            - Остановить все сервисы"
	@echo "  make logs            - Показать логи всех сервисов"
	@echo "  make logs-api        - Показать логи API-сервера"
	@echo "  make logs-worker     - Показать логи worker"
	@echo "  make test            - Запустить тесты"
	@echo "  make help            - Показать эту справку"

build:
	@echo "Сборка API-сервера и worker..."
	mkdir -p bin
	go build $(BUILD_FLAGS) -o $(API_SERVER_BIN) ./cmd/api
	go build $(BUILD_FLAGS) -o $(WORKER_BIN) ./cmd/worker
	@echo "Сборка завершена."

clean:
	@echo "Очистка..."
	rm -rf bin/
	@echo "Очистка завершена."

run-api: build
	@echo "Запуск API-сервера..."
	$(API_SERVER_BIN)

run-worker: build
	@echo "Запуск worker..."
	$(WORKER_BIN)

docker-build:
	@echo "Сборка Docker-образа $(DOCKER_IMAGE_NAME)..."
	docker build -t $(DOCKER_IMAGE_NAME):latest .
	@echo "Сборка Docker-образа завершена."

up:
	@echo "Запуск всех сервисов с Docker Compose..."
	docker-compose -f $(DOCKER_COMPOSE_FILE) up -d --build
	@echo "Сервисы запущены. Используйте 'make logs' для просмотра логов."

up-deps:
	@echo "Запуск зависимостей (PostgreSQL, Redis, Kafka, и т.д.)..."
	docker-compose -f $(DOCKER_COMPOSE_FILE) up -d postgres redis zookeeper kafka prometheus grafana jaeger
	@echo "Зависимости запущены."

up-service:
	@echo "Запуск API-сервера и worker..."
	docker-compose -f $(DOCKER_COMPOSE_FILE) up -d product-api product-worker
	@echo "Сервисы запущены. Используйте 'make logs-api' или 'make logs-worker' для просмотра логов."

down:
	@echo "Остановка всех сервисов..."
	docker-compose -f $(DOCKER_COMPOSE_FILE) down
	@echo "Сервисы остановлены."

logs:
	docker-compose -f $(DOCKER_COMPOSE_FILE) logs -f

logs-api:
	docker-compose -f $(DOCKER_COMPOSE_FILE) logs -f product-api

logs-worker:
	docker-compose -f $(DOCKER_COMPOSE_FILE) logs -f product-worker

test:
	@echo "Запуск тестов..."
	go test -v ./...
	@echo "Тесты завершены."

swagger:
	@echo "Генерация Swagger документации..."
	swag init -g cmd/api/main.go -o ./docs
	@echo "Документация сгенерирована."

keycloak-up:
	@echo "Запуск Keycloak..."
	docker-compose -f ../../deployment/docker-compose/docker-compose.keycloak.yml up -d
	@echo "Keycloak запущен на порту 8085"

keycloak-down:
	@echo "Остановка Keycloak..."
	docker-compose -f deployment/docker-compose/docker-compose.keycloak.yml down
	@echo "Keycloak остановлен"

keycloak-setup:
	@echo "Настройка Keycloak..."
	chmod +x ../../deployment/keycloak/setup/setup-realm.sh
	chmod +x ../../deployment/keycloak/setup/setup-clients.sh
	../../deployment/keycloak/setup/setup-realm.sh
	../../deployment/keycloak/setup/setup-clients.sh
	@echo "Настройка Keycloak завершена"