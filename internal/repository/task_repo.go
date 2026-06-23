package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/jmoiron/sqlx"
)

type taskRepo struct {
	db *sqlx.DB
}

func NewTaskRepository(db *sqlx.DB) TaskRepository {
	return &taskRepo{db: db}
}

func (r *taskRepo) Create(ctx context.Context, t entity.Task) (uint64, error) {
	const q = `
		INSERT INTO tasks
		    (team_id, title, description, status, assignee_id, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, q,
		t.TeamID, t.Title, t.Description, t.Status, t.AssigneeID, t.CreatedBy, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		if parseDuplicate(err) {
			return 0, ErrAlreadyExists
		}
		return 0, fmt.Errorf("tasks.Create: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("tasks.Create.LastInsertId: %w", err)
	}
	return uint64(id), nil
}

func (r *taskRepo) FindByID(ctx context.Context, id uint64) (*entity.Task, error) {

	const q = `
		SELECT id, team_id, title, description, status, assignee_id, created_by, created_at, updated_at, deleted_at
		FROM tasks
		WHERE id = ?
	`
	var t entity.Task
	if err := r.db.GetContext(ctx, &t, q, id); err != nil {
		return nil, wrap("tasks.FindByID", err)
	}
	return &t, nil
}

// buildTaskListWhere — собирает WHERE/args для списка/счётчика задач.
func buildTaskListWhere(f dto.TaskFilter) (string, []any, []string) {
	var (
		conds []string
		args  []any
	)

	if f.TeamID != 0 {
		conds = append(conds, "team_id = ?")
		args = append(args, f.TeamID)
	}
	if f.Status != "" {
		conds = append(conds, "status = ?")
		args = append(args, string(f.Status))
	}
	if f.AssigneeID != nil {
		conds = append(conds, "assignee_id = ?")
		args = append(args, *f.AssigneeID)
	}
	// Все list/count-операции работают только с «живыми» задачами.
	conds = append(conds, "deleted_at IS NULL")

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	return where, args, conds
}

func (r *taskRepo) List(ctx context.Context, f dto.TaskFilter) ([]entity.Task, error) {
	where, args, _ := buildTaskListWhere(f)
	q := fmt.Sprintf(`
		SELECT id, team_id, title, description, status, assignee_id, created_by, created_at, updated_at, deleted_at
		FROM tasks
		%s
		ORDER BY id
		LIMIT ? OFFSET ?
	`, where)
	args = append(args, f.Limit, f.Offset())

	var rows []entity.Task
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, fmt.Errorf("tasks.List: %w", err)
	}
	return rows, nil
}

func (r *taskRepo) Count(ctx context.Context, f dto.TaskFilter) (int, error) {
	where, args, conds := buildTaskListWhere(f)

	if len(conds) == 1 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	q := fmt.Sprintf(`SELECT COUNT(*) FROM tasks %s`, where)
	var n int
	if err := r.db.GetContext(ctx, &n, q, args...); err != nil {
		return 0, fmt.Errorf("tasks.Count: %w", err)
	}
	return n, nil
}

func (r *taskRepo) Update(ctx context.Context, id uint64, patch TaskPatch) (*entity.Task, error) {
	if patch.Title == nil && patch.Description == nil && patch.Status == nil && patch.AssigneeID == nil && !patch.ClearAssignee {

		return r.FindByID(ctx, id)
	}

	set := make([]string, 0, 4)
	args := make([]any, 0, 5)

	if patch.Title != nil {
		set = append(set, "title = ?")
		args = append(args, *patch.Title)
	}
	if patch.Description != nil {
		set = append(set, "description = ?")
		args = append(args, *patch.Description)
	}
	if patch.Status != nil {
		set = append(set, "status = ?")
		args = append(args, string(*patch.Status))
	}
	if patch.ClearAssignee {
		set = append(set, "assignee_id = NULL")
	} else if patch.AssigneeID != nil {
		set = append(set, "assignee_id = ?")
		args = append(args, *patch.AssigneeID)
	}

	if len(set) == 1 {
	}

	set = append(set, "updated_at = CURRENT_TIMESTAMP")

	q := fmt.Sprintf(`UPDATE tasks SET %s WHERE id = ? AND deleted_at IS NULL`, strings.Join(set, ", "))
	args = append(args, id)

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("tasks.Update: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("tasks.Update.RowsAffected: %w", err)
	}
	if n == 0 {

		t, err := r.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if t.DeletedAt != nil {
			return nil, fmt.Errorf("tasks.Update: %w", ErrNotFound)
		}
		return t, nil
	}
	return r.FindByID(ctx, id)
}

func (r *taskRepo) SoftDelete(ctx context.Context, id uint64) error {
	const q = `
		UPDATE tasks
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NULL
	`
	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("tasks.SoftDelete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("tasks.SoftDelete.RowsAffected: %w", err)
	}
	if n == 0 {

		var deletedAt sql.NullTime
		if err := r.db.GetContext(ctx, &deletedAt, `SELECT deleted_at FROM tasks WHERE id = ?`, id); err != nil {
			return wrap("tasks.SoftDelete", err)
		}
		return ErrNotFound
	}
	return nil
}
