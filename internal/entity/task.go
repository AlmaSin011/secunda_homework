package entity

import "time"

type Task struct {
	ID          uint64     `db:"id"`
	TeamID      uint64     `db:"team_id"`
	Title       string     `db:"title"`
	Description *string    `db:"description"`
	Status      TaskStatus `db:"status"`
	AssigneeID  *uint64    `db:"assignee_id"`
	CreatedBy   uint64     `db:"created_by"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}
