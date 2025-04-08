package utils

import (
	"fmt"
	"strings"
	"time"
)

func GenerateConnectionString(
	host, user, password, dbName, sslMode string,
	port, poolSize int,
	timeout time.Duration,
) (string, error) {
	var conStr strings.Builder

	if host == "" {
		return "", ErrStorageEmptyHostName
	}
	if port < 0 || port > 65535 {
		return "", ErrStorageInvalidPortNumber
	}
	if user == "" {
		return "", ErrStorageEmptyUsername
	}
	if password == "" {
		return "", ErrStorageEmptyPassword
	}
	if dbName == "" {
		return "", ErrStorageInvalidDatabaseName
	}
	if sslMode == "" {
		return "", ErrStorageInvalidSslMode
	}
	if timeout < 0 {
		return "", ErrStorageInvalidTimeout
	}
	if poolSize == "" {
		return "", ErrStorageInvalidPoolSize
	}

	conStr.WriteString("host=")
	conStr.WriteString(host)
	conStr.WriteString(" port=")
	conStr.WriteString(fmt.Sprint(port))
	conStr.WriteString(" user=")
	conStr.WriteString(user)
	conStr.WriteString(" password=")
	conStr.WriteString(password)
	conStr.WriteString(" dbname=")
	conStr.WriteString(dbName)
	conStr.WriteString(" sslmode=")
	conStr.WriteString(sslMode)
	conStr.WriteString(" connect_timeout=")
	conStr.WriteString(timeout.String())
	conStr.WriteString(" pool_size=")
	conStr.WriteString(poolSize)

	return conStr.String(), nil
}
