package entity

import "time"

type TeamMember struct {
	UserID   uint64    `db:"user_id"`
	TeamID   uint64    `db:"team_id"`
	Role     Role      `db:"role"`
	JoinedAt time.Time `db:"joined_at"`
}
