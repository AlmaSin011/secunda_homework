package dto

import (
	"strconv"

	"github.com/example/go-project/internal/entity"
)

const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
	MinPageLimit     = 1
)

// TaskFilter — параметры для GET /api/v1/tasks.
type TaskFilter struct {
	TeamID     uint64
	Status     entity.TaskStatus // "" = любой
	AssigneeID *uint64
	Page       int
	Limit      int
}

// BindTaskFilter собирает фильтр из query-параметров Gin.
func BindTaskFilter(get func(string) string) TaskFilter {
	f := TaskFilter{
		Page:  1,
		Limit: DefaultPageLimit,
	}

	if v := get("team_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			f.TeamID = id
		}
	}
	if v := get("assignee_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			f.AssigneeID = &id
		}
	}
	if v := get("status"); v != "" {
		if s, err := entity.ParseTaskStatus(v); err == nil {
			f.Status = s
		}
	}
	if v := get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			f.Page = p
		}
	}
	if v := get("limit"); v != "" {
		// Парсим даже 0, чтобы корректно привести к MinPageLimit.
		if l, err := strconv.Atoi(v); err == nil && l >= 0 {
			f.Limit = l
		}
	}

	if f.Limit > MaxPageLimit {
		f.Limit = MaxPageLimit
	}
	if f.Limit < MinPageLimit {
		f.Limit = MinPageLimit
	}
	return f
}

func (f TaskFilter) Offset() int { return (f.Page - 1) * f.Limit }
