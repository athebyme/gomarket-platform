# Product Service

## Обзор

Product Service - это микросервис для управления продуктами в платформе GoMarket. Сервис предоставляет API для создания, чтения, обновления и удаления информации о продуктах, а также для управления ценами, складскими остатками и категориями продуктов.

## Особенности

- **Мультитенантность**: Поддержка изолированных данных для разных арендаторов (тенантов)
- **Событийная архитектура**: Использование Kafka для асинхронного взаимодействия с другими сервисами
- **Кэширование**: Повышение производительности при помощи Redis
- **Мониторинг**: Интеграция с Prometheus, Grafana и Jaeger
- **Безопасность**: JWT-аутентификация, RBAC и защита от CSRF

## Архитектура

Сервис реализован с использованием принципов гексагональной архитектуры, что обеспечивает четкое разделение бизнес-логики, инфраструктуры и адаптеров.

```
services/product-service/
├── cmd/                     # Точки входа приложения 
│   ├── api/                 # API-сервер
│   └── worker/              # Фоновый обработчик событий
├── config/                  # Файлы конфигурации
├── internal/                # Внутренние пакеты сервиса
│   ├── adapters/            # Адаптеры к внешним системам
│   │   ├── cache/           # Адаптер к Redis
│   │   ├── logger/          # Адаптер для логирования
│   │   ├── messaging/       # Адаптер к Kafka
│   │   └── storage/         # Адаптер к PostgreSQL
│   ├── api/                 # API-слой
│   │   ├── handlers/        # Обработчики HTTP-запросов
│   │   └── middleware/      # Промежуточное ПО
│   ├── domain/              # Бизнес-логика
│   │   ├── models/          # Модели предметной области
│   │   └── services/        # Сервисы предметной области
│   │── security/            # Компоненты безопасности
│   └── utils/               # Вспомогательные функции
├── migrations/              # Миграции базы данных
└── docs/                    # Документация (включая Swagger)
```

## Зависимости

- Go 1.23+
- PostgreSQL 14+
- Redis 7+
- Kafka 3+
- Docker и Docker Compose

## Установка и запуск

### Предварительные требования

- Установленный Go 1.23+
- Docker и Docker Compose
- Доступ к Docker Hub или другому реестру контейнеров

### Клонирование репозитория

```bash
git clone https://github.com/athebyme/gomarket-platform.git
cd gomarket-platform/services/product-service
```

### Запуск с помощью Make

```bash
# Сборка сервиса
make build

# Запуск только зависимостей (PostgreSQL, Redis, Kafka)
make up-deps

# Запуск API-сервера локально
make run-api

# Запуск worker'а локально
make run-worker

# Запуск с Docker Compose (все компоненты)
make up

# Остановка сервисов
make down

# Генерация Swagger-документации
make swagger
```

### Переменные окружения

Основные переменные окружения:

```
# Основные
APP_ENV=development                 # Окружение (development, staging, production)
LOG_LEVEL=debug                    # Уровень логирования

# Сервер
SERVER_PORT=8081                   # Порт API-сервера

# База данных
POSTGRES_HOST=localhost            # Хост PostgreSQL
POSTGRES_PORT=5432                 # Порт PostgreSQL
POSTGRES_USER=postgres             # Пользователь PostgreSQL
POSTGRES_PASSWORD=postgres         # Пароль PostgreSQL
POSTGRES_DBNAME=product_db         # Имя базы данных

# Redis
REDIS_HOST=localhost               # Хост Redis
REDIS_PORT=6379                    # Порт Redis
REDIS_PASSWORD=redis               # Пароль Redis

# Kafka
KAFKA_BROKERS=localhost:9092       # Брокеры Kafka
KAFKA_GROUP_ID=product-service     # ID группы потребителей

# Безопасность
JWT_SECRET=your-secret-key         # Секретный ключ для JWT
```

Полный список переменных окружения можно найти в файле `.env.example`.

## API-документация

API-документация доступна в формате Swagger:

- Swagger UI: `http://localhost:8081/swagger/`
- Swagger JSON: `http://localhost:8081/swagger/doc.json`

Основные эндпоинты:

- `GET /api/v1/products` - Получение списка продуктов
- `POST /api/v1/products` - Создание нового продукта
- `GET /api/v1/products/{id}` - Получение информации о продукте
- `PUT /api/v1/products/{id}` - Обновление продукта
- `DELETE /api/v1/products/{id}` - Удаление продукта
- `POST /api/v1/products/{id}/sync` - Синхронизация продукта с маркетплейсом

## Авторизация

Сервис использует JWT-токены для авторизации. Все API-запросы должны включать заголовок:

```
Authorization: Bearer <token>
```

Токен должен содержать следующие обязательные поля:
- `user_id` - ID пользователя
- `tenant_id` - ID тенанта
- `roles` - Массив ролей пользователя
- `permissions` - Массив разрешений пользователя

## Примеры использования

### Создание продукта

```bash
curl -X POST http://localhost:8081/api/v1/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant1" \
  -H "X-Supplier-ID: supplier1" \
  -d '{
    "id": "",
    "supplier_id": "supplier1",
    "base_data": {
      "name": "Тестовый продукт",
      "description": "Описание тестового продукта",
      "sku": "TEST-001",
      "price": 1000,
      "currency": "RUB"
    },
    "metadata": {
      "source": "manual",
      "tags": ["test", "example"]
    }
  }'
```

### Получение продукта

```bash
curl -X GET http://localhost:8081/api/v1/products/12345 \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant1" \
  -H "X-Supplier-ID: supplier1"
```

## События Kafka

Сервис публикует следующие события:

- `product_created` - Создание продукта
- `product_updated` - Обновление продукта
- `product_deleted` - Удаление продукта
- `product_price_updated` - Обновление цены продукта
- `product_inventory_updated` - Обновление складских остатков

## Мониторинг

Сервис предоставляет метрики Prometheus по адресу `/metrics`.

## Логирование

Логи сервиса можно просмотреть с помощью команд:

```bash
# Просмотр логов всех сервисов
make logs

# Просмотр логов API-сервера
make logs-api

# Просмотр логов worker'а
make logs-worker
```

## Тестирование

```bash
# Запуск всех тестов
make test

# Запуск тестов с отчетом о покрытии
make test-coverage
```

## Troubleshooting

### Проблемы с Docker

Если при сборке Docker-образа возникают проблемы с Alpine 3.20, можно изменить версию Alpine в Dockerfile:

```dockerfile
ARG ALPINE_VERSION=3.19
```


### Проблемы запуска сервера

8080 порт занимает kafka ui. рекомендую ставить в .env порт сервера апи на 8081.

### Ошибки подключения к зависимостям

При запуске сервиса проверьте доступность зависимостей:

```bash
# Проверка PostgreSQL
psql -h localhost -p 5432 -U postgres -d product_db

# Проверка Redis
redis-cli -h localhost -p 6379 ping

# Проверка Kafka
kafka-topics --bootstrap-server localhost:9092 --list
```

## Лицензия

Copyright © 2025 GoMarket Platform