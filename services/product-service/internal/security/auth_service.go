package security

import (
	"context"
	"errors"
)

var ErrInvalidCredentials = errors.New("invalid username or password")
var ErrUserNotFound = errors.New("user not found")

type UserDetails struct {
	UserID      string
	TenantID    string
	Roles       []string
	Permissions []string
}

// AuthServiceInterface defines the contract for authentication operations.
type AuthServiceInterface interface {
	Authenticate(ctx context.Context, username, password string) (*UserDetails, error)
	// RegisterUser (Optional) handles new user registration.
	// RegisterUser(ctx context.Context, /* user data */) error
}

type PlaceholderAuthService struct {
	// In a real app, inject a UserRepository or database connection here
}

func NewPlaceholderAuthService() *PlaceholderAuthService {
	return &PlaceholderAuthService{}
}
