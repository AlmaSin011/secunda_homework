package service

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/example/go-project/internal/repository"
)

// Transactor — абстракция над «выполнить функцию в транзакции».
// Service получает её через интерфейс, чтобы не зависеть от *sqlx.DB в unit-тестах.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(exec repository.DBX) error) error
}

type TxExec = repository.DBX

type sqlxTransactor struct {
	db *sqlx.DB
}

func NewSQLXTransactor(db *sqlx.DB) Transactor {
	return &sqlxTransactor{db: db}
}

func (t *sqlxTransactor) WithinTx(ctx context.Context, fn func(exec repository.DBX) error) error {
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
