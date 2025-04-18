package interfaces

import (
	"context"
)

// AuthPort определяет интерфейс для работы с аутентификацией
type AuthPort interface {
	// ValidateToken проверяет токен и возвращает claims
	ValidateToken(ctx context.Context, token string) (interface{}, error)

	// HasRole проверяет наличие роли у пользователя
	HasRole(claims interface{}, role string) bool

	// HasAnyRole проверяет наличие хотя бы одной роли из списка
	HasAnyRole(claims interface{}, roles ...string) bool
}
