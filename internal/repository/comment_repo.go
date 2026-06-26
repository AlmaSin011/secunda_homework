package repository

import (
	"context"
	"fmt"

	"github.com/example/go-project/internal/entity"
	"github.com/jmoiron/sqlx"
)

type commentRepo struct {
	db *sqlx.DB
}

func NewCommentRepository(db *sqlx.DB) CommentRepository {
	return &commentRepo{db: db}
}

const commentInsertQuery = `
	INSERT INTO task_comments (task_id, user_id, body, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?)
`

func (r *commentRepo) Create(ctx context.Context, c entity.TaskComment) (uint64, error) {
	return r.InsertTx(ctx, nil, c)
}

func (r *commentRepo) InsertTx(ctx context.Context, exec DBX, c entity.TaskComment) (uint64, error) {
	var runner DBX = r.db
	if exec != nil {
		runner = exec
	}
	res, err := runner.ExecContext(ctx, commentInsertQuery,
		c.TaskID, c.UserID, c.Body, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		if parseDuplicate(err) {
			return 0, ErrAlreadyExists
		}
		return 0, fmt.Errorf("task_comments.Insert: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("task_comments.Insert.LastInsertId: %w", err)
	}
	return uint64(id), nil
}

func (r *commentRepo) ListByTask(ctx context.Context, taskID uint64) ([]entity.TaskComment, error) {
	const q = `
		SELECT id, task_id, user_id, body, created_at, updated_at
		FROM task_comments
		WHERE task_id = ?
		ORDER BY id
	`
	var rows []entity.TaskComment
	if err := r.db.SelectContext(ctx, &rows, q, taskID); err != nil {
		return nil, fmt.Errorf("task_comments.ListByTask: %w", err)
	}
	return rows, nil
}
