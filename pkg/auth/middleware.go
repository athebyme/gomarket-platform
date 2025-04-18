// pkg/auth/middleware.go
package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/athebyme/gomarket-platform/pkg/interfaces"
)

// AuthMiddleware промежуточное ПО для проверки JWT токенов
func AuthMiddleware(kc *KeycloakClient, logger interfaces.LoggerPort) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
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

			tokenStr := parts[1]
			claims, err := kc.ValidateToken(r.Context(), tokenStr)
			if err != nil {
				logger.WarnWithContext(r.Context(), "Invalid JWT token",
					interfaces.LogField{Key: "error", Value: err.Error()})
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Добавляем данные из токена в контекст
			ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
			ctx = context.WithValue(ctx, "tenant_id", claims.TenantID)
			ctx = context.WithValue(ctx, "username", claims.Username)
			ctx = context.WithValue(ctx, "email", claims.Email)
			ctx = context.WithValue(ctx, "roles", claims.RealmAccess.Roles)
			ctx = context.WithValue(ctx, "claims", claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole проверяет наличие определенной роли
func RequireRole(kc *KeycloakClient, role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value("claims").(*KeycloakClaims)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !kc.HasRole(claims, role) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyRole проверяет наличие хотя бы одной роли из списка
func RequireAnyRole(kc *KeycloakClient, roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value("claims").(*KeycloakClaims)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !kc.HasAnyRole(claims, roles...) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
