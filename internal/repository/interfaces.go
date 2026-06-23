package repository

import (
	"context"
	"errors"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
)

var (
	ErrNotFound      = errors.New("repository: not found")
	ErrAlreadyExists = errors.New("repository: already exists")
)

type UserRepository interface {
	Create(ctx context.Context, u entity.User) (uint64, error)
	FindByID(ctx context.Context, id uint64) (*entity.User, error)
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
}

type TeamRepository interface {
	Create(ctx context.Context, t entity.Team) (uint64, error)
	FindByID(ctx context.Context, id uint64) (*entity.Team, error)
	ListByUser(ctx context.Context, userID uint64) ([]TeamWithRole, error)

	AddMember(ctx context.Context, m entity.TeamMember) error
	GetMember(ctx context.Context, teamID, userID uint64) (*entity.TeamMember, error)
	ListMembers(ctx context.Context, teamID uint64) ([]entity.TeamMember, error)
	UpdateMemberRole(ctx context.Context, teamID, userID uint64, role entity.Role) error
	RemoveMember(ctx context.Context, teamID, userID uint64) error

	UserBelongsToTeam(ctx context.Context, teamID, userID uint64) (bool, error)
}

type TeamWithRole struct {
	Team entity.Team
	Role entity.Role
}

type TaskRepository interface {
	Create(ctx context.Context, t entity.Task) (uint64, error)

	FindByID(ctx context.Context, id uint64) (*entity.Task, error)

	List(ctx context.Context, f dto.TaskFilter) ([]entity.Task, error)

	Count(ctx context.Context, f dto.TaskFilter) (int, error)

	Update(ctx context.Context, id uint64, patch TaskPatch) (*entity.Task, error)

	SoftDelete(ctx context.Context, id uint64) error
}

type TaskPatch struct {
	Title         *string
	Description   *string
	Status        *entity.TaskStatus
	AssigneeID    *uint64
	ClearAssignee bool //обнулить assignee_id
}

type HistoryRepository interface {
	Insert(ctx context.Context, h entity.TaskHistory) error

	ListByTask(ctx context.Context, taskID uint64) ([]entity.TaskHistory, error)
}

type CommentRepository interface {
	Create(ctx context.Context, c entity.TaskComment) (uint64, error)

	ListByTask(ctx context.Context, taskID uint64) ([]entity.TaskComment, error)
}

type StatsRepository interface {
	TeamStatsLastWeek(ctx context.Context) ([]dto.TeamStatsResponse, error)

	TopCreatorsByTeam(ctx context.Context, sinceDays int, limit int) ([]dto.TopCreatorEntry, error)

	OrphanTasks(ctx context.Context) ([]dto.OrphanTaskResponse, error)
}
