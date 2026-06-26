package service

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// Transactor — абстракция над «выполнить функцию в транзакции».
// Service получает её через интерфейс, чтобы не зависеть от *sqlx.DB в unit-тестах.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(exec TxExec) error) error
}

// TxExec — общий интерфейс над *sqlx.DB и *sqlx.Tx для выполнения запросов.
// Реализуется обоими типами; фейковый Transactor может передать nil.
type TxExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
}

type sqlxTransactor struct {
	db *sqlx.DB
}

func NewSQLXTransactor(db *sqlx.DB) Transactor {
	return &sqlxTransactor{db: db}
}

func (t *sqlxTransactor) WithinTx(ctx context.Context, fn func(exec TxExec) error) error {
	tx, err := t.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
