package utils

import "errors"

// ----------------- storage ------------------
var (
	ErrStorageEmptyHostName       = errors.New("host name is empty")
	ErrStorageInvalidPortNumber   = errors.New("port number is empty")
	ErrStorageEmptyUsername       = errors.New("username is empty")
	ErrStorageEmptyPassword       = errors.New("password is empty")
	ErrStorageInvalidDatabaseName = errors.New("database name is empty")
	ErrStorageInvalidSslMode      = errors.New("SSL mode is invalid")
	ErrStorageInvalidPoolSize     = errors.New("pool size is invalid")
	ErrStorageInvalidTimeout      = errors.New("timeout is invalid")
)

// ----------------- product service ------------------
var (
	ErrInvalidProductId = errors.New("invalid product id")
)
