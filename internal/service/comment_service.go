package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/repository"
)

// CommentService — бизнес-логика комментариев к задачам.
type CommentService struct {
	comments   CommentRepository
	tasks      TaskRepository
	teams      TeamRepository
	transactor Transactor
	now        func() time.Time
}

func NewCommentService(
	comments CommentRepository,
	tasks TaskRepository,
	teams TeamRepository,
	tx Transactor,
) *CommentService {
	return &CommentService{
		comments:   comments,
		tasks:      tasks,
		teams:      teams,
		transactor: tx,
		now:        time.Now,
	}
}

const commentBodyMaxLen = 4000

func (s *CommentService) Create(ctx context.Context, callerID, taskID uint64,
	req dto.CreateCommentRequest) (uint64, error) {

	body := strings.TrimSpace(req.Body)

	if body == "" {
		return 0, ErrValidation
	}
	if len(body) > commentBodyMaxLen {
		return 0, ErrValidation
	}

	t, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	if _, err := s.teams.GetMember(ctx, t.TeamID, callerID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, ErrForbidden
		}
		return 0, err
	}
	c := entity.TaskComment{
		TaskID:    taskID,
		UserID:    callerID,
		Body:      body,
		CreatedAt: s.now(),
		UpdatedAt: s.now(),
	}
	return s.comments.Create(ctx, c)
}

func (s *CommentService) List(ctx context.Context, callerID, taskID uint64) ([]dto.CommentResponse, error) {
	t, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if _, err := s.teams.GetMember(ctx, t.TeamID, callerID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrForbidden
		}
		return nil, err
	}
	rows, err := s.comments.ListByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.CommentResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, dto.CommentResponse{
			ID:        c.ID,
			TaskID:    c.TaskID,
			UserID:    c.UserID,
			Body:      c.Body,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
			UpdatedAt: c.UpdatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}
