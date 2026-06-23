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

func (r *historyRepo) Insert(ctx context.Context, h entity.TaskHistory) error {
	const q = `
		INSERT INTO task_history (task_id, changed_by, field, old_value, new_value, changed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	if _, err := r.db.ExecContext(ctx, q,
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
