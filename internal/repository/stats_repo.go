package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/example/go-project/internal/dto"
	"github.com/jmoiron/sqlx"
)

type statsRepo struct {
	db  *sqlx.DB
	now func() time.Time
}

func NewStatsRepository(db *sqlx.DB) StatsRepository {
	return &statsRepo{db: db, now: time.Now}
}

func (r *statsRepo) TeamStatsLastWeek(ctx context.Context) ([]dto.TeamStatsResponse, error) {
	const query = `
		SELECT
		    t.id              AS team_id,
		    t.name            AS team_name,
		    (
		        SELECT COUNT(*)
		        FROM team_members tm
		        WHERE tm.team_id = t.id
		    )                 AS member_count,
		    (
		        SELECT COUNT(*)
		        FROM task_history th
		        INNER JOIN tasks tk ON tk.id = th.task_id
		        WHERE tk.team_id = t.id
		          AND th.field = 'status'
		          AND th.new_value = 'done'
		          AND th.changed_at >= ?
		    )                 AS done_last_7_days
		FROM teams t
		ORDER BY t.id
	`
	weekAgo := r.now().Add(-7 * 24 * time.Hour)
	var rows []dto.TeamStatsResponse
	if err := r.db.SelectContext(ctx, &rows, query, weekAgo); err != nil {
		return nil, fmt.Errorf("stats.TeamStatsLastWeek: %w", err)
	}
	return rows, nil
}

func (r *statsRepo) TopCreatorsByTeam(ctx context.Context, sinceDays int, limit int) ([]dto.TopCreatorEntry, error) {
	if sinceDays <= 0 {
		sinceDays = 30
	}
	if limit <= 0 {
		limit = 3
	}

	const query = `
		WITH ranked AS (
		    SELECT
		        tk.team_id,
		        t.name                 AS team_name,
		        tk.created_by          AS user_id,
		        u.name                 AS user_name,
		        u.email                AS user_email,
		        COUNT(*)               AS task_count,
		        ROW_NUMBER() OVER (
		            PARTITION BY tk.team_id
		            ORDER BY COUNT(*) DESC, tk.created_by ASC
		        )                      AS rnk,
		        ?                      AS window_from,
		        ?                      AS window_to
		    FROM tasks tk
		    INNER JOIN teams t ON t.id = tk.team_id
		    INNER JOIN users u ON u.id = tk.created_by
		    WHERE tk.deleted_at IS NULL
		      AND tk.created_at >= ?
		    GROUP BY tk.team_id, t.name, tk.created_by, u.name, u.email
		)
		SELECT
		    team_id, team_name, user_id, user_name, user_email,
		    task_count, rnk AS ` + "`rank`" + `,
		    window_from, window_to
		FROM ranked
		WHERE rnk <= ?
		ORDER BY team_id, rnk
	`

	now := r.now()
	from := now.Add(-time.Duration(sinceDays) * 24 * time.Hour)

	var rows []dto.TopCreatorEntry
	if err := r.db.SelectContext(ctx, &rows, query, from, now, from, limit); err != nil {
		return nil, fmt.Errorf("stats.TopCreatorsByTeam: %w", err)
	}
	return rows, nil
}

func (r *statsRepo) OrphanTasks(ctx context.Context) ([]dto.OrphanTaskResponse, error) {
	const query = `
		SELECT
		    tk.id                AS task_id,
		    tk.team_id           AS team_id,
		    tk.title             AS title,
		    tk.assignee_id       AS assignee_id,
		    u.email              AS assignee_email
		FROM tasks tk
		LEFT JOIN users u ON u.id = tk.assignee_id
		WHERE tk.deleted_at IS NULL
		  AND tk.assignee_id IS NOT NULL
		  AND NOT EXISTS (
		      SELECT 1
		      FROM team_members tm
		      WHERE tm.team_id = tk.team_id
		        AND tm.user_id = tk.assignee_id
		  )
		ORDER BY tk.team_id, tk.id
	`
	var rows []dto.OrphanTaskResponse
	if err := r.db.SelectContext(ctx, &rows, query); err != nil {
		return nil, fmt.Errorf("stats.OrphanTasks: %w", err)
	}
	return rows, nil
}
