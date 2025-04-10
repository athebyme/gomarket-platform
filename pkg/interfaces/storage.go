package interfaces

import (
	"context"
)

// StoragePort определяет интерфейс для работы с постоянным хранилищем данных
// Реализация может использовать любую базу данных (PostgreSQL, MySQL, MongoDB и т.д.)
type StoragePort interface {
	// BeginTx начинает новую транзакцию
	BeginTx(ctx context.Context) (context.Context, error)

	// CommitTx фиксирует транзакцию
	CommitTx(ctx context.Context) error

	// RollbackTx откатывает транзакцию
	RollbackTx(ctx context.Context) error

	// Метод закрытия соединения

	// Close закрывает соединение с хранилищем
	Close() error
}
