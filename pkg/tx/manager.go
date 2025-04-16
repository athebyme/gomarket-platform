package tx

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// txKey - ключ для хранения транзакции в контексте. Используем приватный тип, чтобы избежать коллизий.
type txKeyType struct{}

var txKey = txKeyType{}

// TxManager управляет жизненным циклом транзакций БД.
type TxManager interface {
	// Do выполняет переданную функцию `fn` внутри транзакции.
	// Если `fn` возвращает ошибку, транзакция откатывается (Rollback).
	// Если `fn` завершается успешно (возвращает nil), транзакция фиксируется (Commit).
	// Контекст, передаваемый в `fn`, будет содержать саму транзакцию.
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// pgxTxManager - реализация TxManager для pgx.
type pgxTxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager создает новый менеджер транзакций.
func NewTxManager(pool *pgxpool.Pool) TxManager {
	return &pgxTxManager{pool: pool}
}

// Do реализует метод интерфейса TxManager.
func (m *pgxTxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	// Начинаем транзакцию
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("tx.Begin failed: %w", err)
	}

	// Создаем новый контекст с транзакцией внутри
	txCtx := context.WithValue(ctx, txKey, tx)

	// Гарантируем откат транзакции в случае паники внутри fn или ошибки при коммите
	// Rollback вернет ошибку только если транзакция уже была завершена (скоммичена или откатана)
	// или если соединение потеряно. В большинстве случаев ошибка здесь не так важна,
	// как ошибка от fn или Commit.
	defer func() {
		// Мы используем явный rollback в блоке ошибки fn,
		// но defer нужен для случаев паники или если Commit вернет ошибку.
		_ = tx.Rollback(ctx)
	}()

	// Выполняем переданную функцию с контекстом, содержащим транзакцию
	err = fn(txCtx)
	if err != nil {
		// Если функция вернула ошибку, откатываем транзакцию
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			// Логируем ошибку отката, но возвращаем оригинальную ошибку от fn
			fmt.Printf("WARNING: failed to rollback tx after error: %v (original error: %v)\n", rollbackErr, err)
			// TODO: Заменить Printf на логгер
		}
		// Возвращаем ошибку, которую вернула fn
		return err
	}

	// Если функция завершилась успешно, коммитим транзакцию
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("tx.Commit failed: %w", err)
	}

	// Все прошло успешно
	return nil
}

// GetTxFromContext извлекает транзакцию из контекста.
// Эта функция может использоваться ВНУТРИ блока fn, переданного в TxManager.Do,
// если нужно получить объект транзакции напрямую (хотя обычно это не требуется,
// если репозиторий использует тот же ключ контекста).
func GetTxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey).(pgx.Tx)
	return tx, ok
}

func GetKey() interface{} {
	return txKey
}
