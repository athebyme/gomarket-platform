package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/patrickmn/go-cache"
	"golang.org/x/oauth2"
)

// KeycloakConfig конфигурация для Keycloak
type KeycloakConfig struct {
	ServerURL    string
	Realm        string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// KeycloakClaims представляет собой структуру claims из токена Keycloak
type KeycloakClaims struct {
	UserID      string `json:"sub"`
	Username    string `json:"preferred_username"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	TenantID    string `json:"tenant_id"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
	ResourceAccess map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access"`
}

// KeycloakClient клиент для работы с Keycloak
type KeycloakClient struct {
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	tokenCache   *cache.Cache
	clientID     string
	realm        string
	serverURL    string
}

// NewKeycloakClient создает новый клиент Keycloak
func NewKeycloakClient(cfg KeycloakConfig) (*KeycloakClient, error) {
	ctx := context.Background()

	providerURL := fmt.Sprintf("%s/realms/%s", cfg.ServerURL, cfg.Realm)

	provider, err := oidc.NewProvider(ctx, providerURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания OIDC провайдера: %w", err)
	}

	oauth2Config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID:        cfg.ClientID,
		SkipIssuerCheck: true,
	})

	tokenCache := cache.New(5*time.Minute, 10*time.Minute)

	return &KeycloakClient{
		provider:     provider,
		verifier:     verifier,
		oauth2Config: oauth2Config,
		tokenCache:   tokenCache,
		clientID:     cfg.ClientID,
		realm:        cfg.Realm,
		serverURL:    cfg.ServerURL,
	}, nil
}

// ValidateToken проверяет JWT токен и возвращает claims
func (k *KeycloakClient) ValidateToken(ctx context.Context, tokenString string) (*KeycloakClaims, error) {
	if cachedClaims, found := k.tokenCache.Get(tokenString); found {
		return cachedClaims.(*KeycloakClaims), nil
	}

	idToken, err := k.verifier.Verify(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("ошибка верификации токена: %w", err)
	}

	var claims KeycloakClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("ошибка извлечения claims: %w", err)
	}

	expiresIn := time.Until(idToken.Expiry)
	if expiresIn > 0 {
		k.tokenCache.Set(tokenString, &claims, expiresIn)
	}

	return &claims, nil
}

// HasRole проверяет наличие роли у пользователя
func (k *KeycloakClient) HasRole(claims *KeycloakClaims, role string) bool {
	for _, r := range claims.RealmAccess.Roles {
		if r == role {
			return true
		}
	}

	if clientRoles, exists := claims.ResourceAccess[k.clientID]; exists {
		for _, r := range clientRoles.Roles {
			if r == role {
				return true
			}
		}
	}

	return false
}

// HasAnyRole проверяет наличие хотя бы одной роли из списка
func (k *KeycloakClient) HasAnyRole(claims *KeycloakClaims, roles ...string) bool {
	for _, role := range roles {
		if k.HasRole(claims, role) {
			return true
		}
	}
	return false
}

// GetAuthURL возвращает URL для аутентификации пользователя
func (k *KeycloakClient) GetAuthURL(state string) string {
	return k.oauth2Config.AuthCodeURL(state)
}

// ExchangeCode обменивает код авторизации на токены
func (k *KeycloakClient) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return k.oauth2Config.Exchange(ctx, code)
}

// GetUserInfo получает информацию о пользователе
func (k *KeycloakClient) GetUserInfo(ctx context.Context, token *oauth2.Token) (*KeycloakClaims, error) {
	userInfo, err := k.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, fmt.Errorf("ошибка получения информации о пользователе: %w", err)
	}

	var claims KeycloakClaims
	if err := userInfo.Claims(&claims); err != nil {
		return nil, fmt.Errorf("ошибка извлечения claims: %w", err)
	}

	return &claims, nil
}
