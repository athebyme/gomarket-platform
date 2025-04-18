package config

import (
	"github.com/athebyme/gomarket-platform/pkg/auth"
)

// KeycloakConfig представляет конфигурацию Keycloak для product-service
type KeycloakConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	ServerURL    string `mapstructure:"server_url"`
	Realm        string `mapstructure:"realm"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
}

// GetKeycloakConfig возвращает конфигурацию для auth.KeycloakClient
func (k *KeycloakConfig) GetKeycloakConfig() auth.KeycloakConfig {
	return auth.KeycloakConfig{
		ServerURL:    k.ServerURL,
		Realm:        k.Realm,
		ClientID:     k.ClientID,
		ClientSecret: k.ClientSecret,
		RedirectURL:  "",
	}
}
