package entity

import "time"

type User struct {
	ID           uint64    `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"-"` // не сериализуем в JSON
	Name         string    `db:"name"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}
