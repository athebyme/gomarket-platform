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

echo "Создание realm ${REALM_NAME}..."
curl -s -X POST \
  "${KEYCLOAK_URL}/admin/realms" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"realm\": \"${REALM_NAME}\",
    \"enabled\": true,
    \"displayName\": \"GoMarket Platform\",
    \"accessTokenLifespan\": 900,
    \"sslRequired\": \"external\",
    \"registrationAllowed\": false,
    \"resetPasswordAllowed\": true,
    \"editUsernameAllowed\": false,
    \"duplicateEmailsAllowed\": false
  }"

echo "Создание базовых ролей..."
curl -s -X POST \
  "${KEYCLOAK_URL}/admin/realms/${REALM_NAME}/roles" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"admin\",
    \"description\": \"Администратор платформы\"
  }"

curl -s -X POST \
  "${KEYCLOAK_URL}/admin/realms/${REALM_NAME}/roles" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"manager\",
    \"description\": \"Менеджер\"
  }"

curl -s -X POST \
  "${KEYCLOAK_URL}/admin/realms/${REALM_NAME}/roles" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"user\",
    \"description\": \"Обычный пользователь\"
  }"

echo "Realm ${REALM_NAME} успешно создан!"