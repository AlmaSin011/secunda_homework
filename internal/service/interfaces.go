package service

import (
	"context"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/repository"
)

type UserLookup interface {
	FindByID(ctx context.Context, id uint64) (*entity.User, error)
}

type TeamRepository interface {
	Create(ctx context.Context, t entity.Team) (uint64, error)
	FindByID(ctx context.Context, id uint64) (*entity.Team, error)
	ListByUser(ctx context.Context, userID uint64) ([]repository.TeamWithRole, error)
	AddMember(ctx context.Context, m entity.TeamMember) error
	GetMember(ctx context.Context, teamID, userID uint64) (*entity.TeamMember, error)
	ListMembers(ctx context.Context, teamID uint64) ([]entity.TeamMember, error)
}

type TaskRepository interface {
	Create(ctx context.Context, t entity.Task) (uint64, error)
	FindByID(ctx context.Context, id uint64) (*entity.Task, error)
	List(ctx context.Context, f dto.TaskFilter) ([]entity.Task, error)
	Count(ctx context.Context, f dto.TaskFilter) (int, error)
	Update(ctx context.Context, id uint64, patch repository.TaskPatch) (*entity.Task, error)
	UpdateTx(ctx context.Context, exec TxExec, id uint64, patch repository.TaskPatch) (*entity.Task, error)
	SoftDelete(ctx context.Context, id uint64) error
}

type HistoryRepository interface {
	Insert(ctx context.Context, h entity.TaskHistory) error
	InsertTx(ctx context.Context, exec TxExec, h entity.TaskHistory) error
	ListByTask(ctx context.Context, taskID uint64) ([]entity.TaskHistory, error)
}

type CommentRepository interface {
	Create(ctx context.Context, c entity.TaskComment) (uint64, error)
	InsertTx(ctx context.Context, exec TxExec, c entity.TaskComment) (uint64, error)
	ListByTask(ctx context.Context, taskID uint64) ([]entity.TaskComment, error)
}

type StatsRepository interface {
	TeamStatsLastWeek(ctx context.Context) ([]dto.TeamStatsResponse, error)
	TopCreatorsByTeam(ctx context.Context, sinceDays int, limit int) ([]dto.TopCreatorEntry, error)
	OrphanTasks(ctx context.Context) ([]dto.OrphanTaskResponse, error)
}
