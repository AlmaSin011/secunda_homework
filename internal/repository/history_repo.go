package repository

import (
	"context"
	"fmt"

	"github.com/example/go-project/internal/entity"
	"github.com/jmoiron/sqlx"
)

type historyRepo struct {
	db *sqlx.DB
}

func NewHistoryRepository(db *sqlx.DB) HistoryRepository {
	return &historyRepo{db: db}
}

const historyInsertQuery = `
	INSERT INTO task_history (task_id, changed_by, field, old_value, new_value, changed_at)
	VALUES (?, ?, ?, ?, ?, ?)
`

func (r *historyRepo) Insert(ctx context.Context, h entity.TaskHistory) error {
	return r.InsertTx(ctx, nil, h)
}

func (r *historyRepo) InsertTx(ctx context.Context, exec DBX, h entity.TaskHistory) error {
	var runner DBX = r.db
	if exec != nil {
		runner = exec
	}
	if _, err := runner.ExecContext(ctx, historyInsertQuery,
		h.TaskID, h.ChangedBy, h.Field, h.OldValue, h.NewValue, h.ChangedAt,
	); err != nil {
		return fmt.Errorf("task_history.Insert: %w", err)
	}
	return nil
}

func (r *historyRepo) ListByTask(ctx context.Context, taskID uint64) ([]entity.TaskHistory, error) {
	const q = `
		SELECT id, task_id, changed_by, field, old_value, new_value, changed_at
		FROM task_history
		WHERE task_id = ?
		ORDER BY id
	`
	var rows []entity.TaskHistory
	if err := r.db.SelectContext(ctx, &rows, q, taskID); err != nil {
		return nil, fmt.Errorf("task_history.ListByTask: %w", err)
	}
	return rows, nil
}
