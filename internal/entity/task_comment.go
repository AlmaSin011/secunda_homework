package entity

import "time"

type TaskComment struct {
	ID        uint64    `db:"id"`
	TaskID    uint64    `db:"task_id"`
	UserID    uint64    `db:"user_id"`
	Body      string    `db:"body"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
