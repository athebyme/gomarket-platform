export KEYCLOAK_URL="http://localhost:8085"
export REALM_NAME="gomarket"
export CLIENT_ID="gomarket-frontend"
export USERNAME="bob"
export PASSWORD="123"

ACCESS_TOKEN=$(curl -X POST \
  "${KEYCLOAK_URL}/realms/${REALM_NAME}/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=${CLIENT_ID}" \
  -d "username=${USERNAME}" \
  -d "password=${PASSWORD}" \
  -d "grant_type=password")

if [ -z "$ACCESS_TOKEN" ] || [ "$ACCESS_TOKEN" == "null" ]; then
    echo "Ошибка: Не удалось получить токен. Проверь URL, realm, client_id, имя пользователя и пароль."
else
    echo "Токен успешно получен:"
    echo "$ACCESS_TOKEN"
fi

if [ ! -z "$ACCESS_TOKEN" ] && [ "$ACCESS_TOKEN" != "null" ]; then
  echo ""
  echo "Содержимое токена (payload):"
  echo $ACCESS_TOKEN | jq -R 'split(".") | .[1] | @base64d | fromjson' 2>/dev/null || echo "Не удалось декодировать payload"
fi
