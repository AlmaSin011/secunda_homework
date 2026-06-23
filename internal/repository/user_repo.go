package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/example/go-project/internal/entity"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type userRepo struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepo{db: db}
}

const (
	mysqlErrDuplicateEntry = 1062
)

func parseDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == mysqlErrDuplicateEntry
}

func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", op, err)
}

func (r *userRepo) Create(ctx context.Context, u entity.User) (uint64, error) {
	const q = `
		INSERT INTO users (email, password_hash, name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, q,
		strings.ToLower(strings.TrimSpace(u.Email)),
		u.PasswordHash,
		strings.TrimSpace(u.Name),
		u.CreatedAt,
		u.UpdatedAt,
	)
	if err != nil {
		if parseDuplicate(err) {
			return 0, ErrAlreadyExists
		}
		return 0, fmt.Errorf("users.Create: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("users.Create.LastInsertId: %w", err)
	}
	return uint64(id), nil
}

func (r *userRepo) FindByID(ctx context.Context, id uint64) (*entity.User, error) {
	const q = `
		SELECT id, email, password_hash, name, created_at, updated_at
		FROM users
		WHERE id = ?
	`
	var u entity.User
	if err := r.db.GetContext(ctx, &u, q, id); err != nil {
		return nil, wrap("users.FindByID", err)
	}
	return &u, nil
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	const q = `
		SELECT id, email, password_hash, name, created_at, updated_at
		FROM users
		WHERE email = ?
	`
	var u entity.User
	if err := r.db.GetContext(ctx, &u, q, strings.ToLower(strings.TrimSpace(email))); err != nil {

		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("users.FindByEmail: %w", err)
	}
	return &u, nil
}
