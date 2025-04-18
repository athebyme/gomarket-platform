#!/bin/bash

KEYCLOAK_URL=${KEYCLOAK_URL:-"http://localhost:8085"}
ADMIN_USER=${KEYCLOAK_ADMIN:-"admin"}
ADMIN_PASSWORD=${KEYCLOAK_ADMIN_PASSWORD:-"admin"}
REALM_NAME="gomarket"

echo "Получение токена администратора..."
ADMIN_TOKEN=$(curl -s -X POST \
  "${KEYCLOAK_URL}/realms/master/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=${ADMIN_USER}" \
  -d "password=${ADMIN_PASSWORD}" \
  -d "grant_type=password" \
  -d "client_id=admin-cli" | jq -r '.access_token')

if [ -z "$ADMIN_TOKEN" ] || [ "$ADMIN_TOKEN" == "null" ]; then
    echo "Ошибка получения токена администратора"
    exit 1
fi

echo "Создание клиента product-service..."
curl -s -X POST \
  "${KEYCLOAK_URL}/admin/realms/${REALM_NAME}/clients" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"clientId\": \"product-service\",
    \"enabled\": true,
    \"bearerOnly\": false,
    \"publicClient\": false,
    \"standardFlowEnabled\": false,
    \"directAccessGrantsEnabled\": false,
    \"serviceAccountsEnabled\": true,
    \"authorizationServicesEnabled\": true,
    \"secret\": \"product-service-secret\"
  }"

echo "Создание клиента marketplace-service..."
curl -s -X POST \
  "${KEYCLOAK_URL}/admin/realms/${REALM_NAME}/clients" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"clientId\": \"marketplace-service\",
    \"enabled\": true,
    \"bearerOnly\": true,
    \"publicClient\": false,
    \"standardFlowEnabled\": false,
    \"directAccessGrantsEnabled\": false,
    \"serviceAccountsEnabled\": true,
    \"authorizationServicesEnabled\": true,
    \"secret\": \"marketplace-service-secret\"
  }"

echo "Создание клиента supplier-service..."
curl -s -X POST \
  "${KEYCLOAK_URL}/admin/realms/${REALM_NAME}/clients" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"clientId\": \"supplier-service\",
    \"enabled\": true,
    \"bearerOnly\": true,
    \"publicClient\": false,
    \"standardFlowEnabled\": false,
    \"directAccessGrantsEnabled\": false,
    \"serviceAccountsEnabled\": true,
    \"authorizationServicesEnabled\": true,
    \"secret\": \"supplier-service-secret\"
  }"

echo "Создание клиента frontend..."
curl -s -X POST \
  "${KEYCLOAK_URL}/admin/realms/${REALM_NAME}/clients" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"clientId\": \"gomarket-frontend\",
    \"enabled\": true,
    \"bearerOnly\": false,
    \"publicClient\": true,
    \"standardFlowEnabled\": true,
    \"implicitFlowEnabled\": false,
    \"directAccessGrantsEnabled\": true,
    \"serviceAccountsEnabled\": false,
    \"redirectUris\": [\"http://localhost:3000/*\"],
    \"webOrigins\": [\"http://localhost:3000\"]
  }"

echo "Клиенты успешно созданы!"