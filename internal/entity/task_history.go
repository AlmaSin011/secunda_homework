package entity

import "time"

type TaskHistory struct {
	ID        uint64    `db:"id"`
	TaskID    uint64    `db:"task_id"`
	ChangedBy uint64    `db:"changed_by"`
	Field     string    `db:"field"`
	OldValue  *string   `db:"old_value"`
	NewValue  *string   `db:"new_value"`
	ChangedAt time.Time `db:"changed_at"`
}
