package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/example/go-project/internal/entity"
	"github.com/jmoiron/sqlx"
)

type teamRepo struct {
	db *sqlx.DB
}

func NewTeamRepository(db *sqlx.DB) TeamRepository {
	return &teamRepo{db: db}
}

func (r *teamRepo) Create(ctx context.Context, t entity.Team) (uint64, error) {
	const q = `
		INSERT INTO teams (name, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, q, t.Name, t.CreatedBy, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		if parseDuplicate(err) {
			return 0, ErrAlreadyExists
		}
		return 0, fmt.Errorf("teams.Create: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("teams.Create.LastInsertId: %w", err)
	}
	return uint64(id), nil
}

func (r *teamRepo) FindByID(ctx context.Context, id uint64) (*entity.Team, error) {
	const q = `
		SELECT id, name, created_by, created_at, updated_at
		FROM teams
		WHERE id = ?
	`
	var t entity.Team
	if err := r.db.GetContext(ctx, &t, q, id); err != nil {
		return nil, wrap("teams.FindByID", err)
	}
	return &t, nil
}

func (r *teamRepo) ListByUser(ctx context.Context, userID uint64) ([]TeamWithRole, error) {
	const q = `
		SELECT t.id, t.name, t.created_by, t.created_at, t.updated_at,
		       tm.role
		FROM teams t
		INNER JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = ?
		ORDER BY t.id
	`
	type row struct {
		entity.Team
		Role entity.Role `db:"role"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, userID); err != nil {
		return nil, fmt.Errorf("teams.ListByUser: %w", err)
	}
	out := make([]TeamWithRole, 0, len(rows))
	for _, x := range rows {
		out = append(out, TeamWithRole{Team: x.Team, Role: x.Role})
	}
	return out, nil
}

func (r *teamRepo) AddMember(ctx context.Context, m entity.TeamMember) error {
	const q = `
		INSERT INTO team_members (user_id, team_id, role, joined_at)
		VALUES (?, ?, ?, ?)
	`
	if _, err := r.db.ExecContext(ctx, q, m.UserID, m.TeamID, m.Role, m.JoinedAt); err != nil {
		if parseDuplicate(err) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("team_members.AddMember: %w", err)
	}
	return nil
}

func (r *teamRepo) GetMember(ctx context.Context, teamID, userID uint64) (*entity.TeamMember, error) {
	const q = `
		SELECT user_id, team_id, role, joined_at
		FROM team_members
		WHERE team_id = ? AND user_id = ?
	`
	var m entity.TeamMember
	if err := r.db.GetContext(ctx, &m, q, teamID, userID); err != nil {
		return nil, wrap("team_members.GetMember", err)
	}
	return &m, nil
}

func (r *teamRepo) ListMembers(ctx context.Context, teamID uint64) ([]entity.TeamMember, error) {
	const q = `
		SELECT user_id, team_id, role, joined_at
		FROM team_members
		WHERE team_id = ?
		ORDER BY joined_at, user_id
	`
	var ms []entity.TeamMember
	if err := r.db.SelectContext(ctx, &ms, q, teamID); err != nil {
		return nil, fmt.Errorf("team_members.ListMembers: %w", err)
	}
	return ms, nil
}

func (r *teamRepo) UpdateMemberRole(ctx context.Context, teamID, userID uint64, role entity.Role) error {
	const q = `
		UPDATE team_members
		SET role = ?
		WHERE team_id = ? AND user_id = ?
	`
	res, err := r.db.ExecContext(ctx, q, role, teamID, userID)
	if err != nil {
		return fmt.Errorf("team_members.UpdateMemberRole: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("team_members.UpdateMemberRole.RowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *teamRepo) RemoveMember(ctx context.Context, teamID, userID uint64) error {
	const q = `DELETE FROM team_members WHERE team_id = ? AND user_id = ?`
	res, err := r.db.ExecContext(ctx, q, teamID, userID)
	if err != nil {
		return fmt.Errorf("team_members.RemoveMember: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("team_members.RemoveMember.RowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *teamRepo) UserBelongsToTeam(ctx context.Context, teamID, userID uint64) (bool, error) {
	const q = `
		SELECT 1
		FROM team_members
		WHERE team_id = ? AND user_id = ?
		LIMIT 1
	`
	var one int
	if err := r.db.GetContext(ctx, &one, q, teamID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("team_members.UserBelongsToTeam: %w", err)
	}
	return true, nil
}
