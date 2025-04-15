package utils

import (
	"strconv"
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
	if poolSize < 0 {
		return "", ErrStorageInvalidPoolSize
	}

	conStr.WriteString("host=")
	conStr.WriteString(host)
	conStr.WriteString(" port=")
	conStr.WriteString(strconv.Itoa(port))
	conStr.WriteString(" user=")
	conStr.WriteString(user)
	conStr.WriteString(" password=")
	conStr.WriteString(password)
	conStr.WriteString(" dbname=")
	conStr.WriteString(dbName)
	conStr.WriteString(" sslmode=")
	conStr.WriteString(sslMode)

	if timeout > 0 {
		conStr.WriteString(" connect_timeout=")
		conStr.WriteString(strconv.Itoa(int(timeout.Seconds())))
	}

	return conStr.String(), nil
}
