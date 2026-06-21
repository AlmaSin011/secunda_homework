package entity

import "time"

type Team struct {
	ID        uint64    `db:"id"`
	Name      string    `db:"name"`
	CreatedBy uint64    `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
