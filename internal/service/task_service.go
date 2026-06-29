package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/example/go-project/internal/cache"
	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/repository"
)

// TaskService — бизнес-логика задач.
type TaskService struct {
	tasks      TaskRepository
	history    HistoryRepository
	teams      TeamRepository
	cache      cache.Cache
	transactor Transactor
	now        func() time.Time
}

func NewTaskService(
	tasks TaskRepository,
	history HistoryRepository,
	teams TeamRepository,
	cache cache.Cache,
	tx Transactor,
) *TaskService {
	return &TaskService{
		tasks:      tasks,
		history:    history,
		teams:      teams,
		cache:      cache,
		transactor: tx,
		now:        time.Now,
	}
}

func (s *TaskService) taskCache() *taskListCache {
	return newTaskListCache(s.cache)
}

func (s *TaskService) Create(ctx context.Context, actorID uint64, req dto.CreateTaskRequest) (*entity.Task, error) {

	if req.Title == "" || strings.TrimSpace(req.Title) == "" {
		return nil, ErrValidation
	}
	if len(req.Title) > 255 {
		return nil, ErrValidation
	}
	if !req.Status.IsValid() || req.Status == "" {
		return nil, ErrValidation
	}
	if req.TeamID == 0 {
		return nil, ErrValidation
	}
	// Членство проверяется через GetMember.
	mem, err := s.teams.GetMember(ctx, req.TeamID, actorID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrForbidden
		}
		return nil, err
	}
	_ = mem

	t := entity.Task{
		TeamID:      req.TeamID,
		Title:       strings.TrimSpace(req.Title),
		Description: req.Description,
		Status:      req.Status,
		AssigneeID:  req.AssigneeID,
		CreatedBy:   actorID,
		CreatedAt:   s.now(),
		UpdatedAt:   s.now(),
	}
	id, err := s.tasks.Create(ctx, t)
	if err != nil {
		return nil, err
	}
	t.ID = id

	if err := s.taskCache().Invalidate(ctx, t.TeamID); err != nil {
		// Не критично: данные корректны, кеш «протухнет» через TTL.
		_ = err
	}
	return &t, nil
}

func (s *TaskService) Get(ctx context.Context, callerID, taskID uint64) (*entity.Task, error) {
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
	return t, nil
}

func (s *TaskService) List(ctx context.Context, callerID uint64, f dto.TaskFilter) (*dto.TasksListResponse, error) {
	if f.TeamID == 0 {
		return nil, ErrValidation
	}
	if _, err := s.teams.GetMember(ctx, f.TeamID, callerID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrForbidden
		}
		return nil, err
	}

	if f.Status != "" && !f.Status.IsValid() {
		return nil, ErrValidation
	}

	if cached, err := s.taskCache().Get(ctx, f); err == nil && cached != nil {
		return cached, nil
	}

	rows, err := s.tasks.List(ctx, f)
	if err != nil {
		return nil, err
	}
	total, err := s.tasks.Count(ctx, f)
	if err != nil {
		return nil, err
	}

	resp := dto.TasksListResponse{
		Items: make([]dto.TaskResponse, 0, len(rows)),
		Meta: dto.Pagination{
			Page:  f.Page,
			Limit: f.Limit,
			Total: total,
		},
	}
	for _, t := range rows {
		resp.Items = append(resp.Items, toTaskResponse(t))
	}

	if err := s.taskCache().Store(ctx, f, &resp); err != nil {
		_ = err
	}
	return &resp, nil
}

func (s *TaskService) Update(ctx context.Context, actorID, taskID uint64, req dto.UpdateTaskRequest) (*entity.Task, error) {
	patch, patchOK, hasChanges := buildPatchRich(req)
	if !patchOK {
		return nil, ErrValidation
	}
	if !hasChanges {
		return s.Get(ctx, actorID, taskID)
	}

	// Проверяем, что задача существует и actor — член команды.
	existing, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if _, err := s.teams.GetMember(ctx, existing.TeamID, actorID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrForbidden
		}
		return nil, err
	}

	var updated *entity.Task
	now := s.now()

	err = s.transactor.WithinTx(ctx, func(exec TxExec) error {
		u, err := s.tasks.UpdateTx(ctx, exec, taskID, patch)
		if err != nil {
			return err
		}
		updated = u

		entries := buildHistoryEntries(taskID, actorID, now, existing, u, patch)
		for _, h := range entries {
			if err := s.history.InsertTx(ctx, exec, h); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := s.taskCache().Invalidate(ctx, existing.TeamID); err != nil {
		_ = err
	}
	return updated, nil
}

func (s *TaskService) Delete(ctx context.Context, actorID, taskID uint64) error {
	t, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if _, err := s.teams.GetMember(ctx, t.TeamID, actorID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrForbidden
		}
		return err
	}
	if err := s.tasks.SoftDelete(ctx, taskID); err != nil {
		return err
	}
	if err := s.taskCache().Invalidate(ctx, t.TeamID); err != nil {
		_ = err
	}
	return nil
}

func (s *TaskService) History(ctx context.Context, callerID, taskID uint64) ([]dto.TaskHistoryResponse, error) {
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
	rows, err := s.history.ListByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.TaskHistoryResponse, 0, len(rows))
	for _, h := range rows {
		out = append(out, dto.TaskHistoryResponse{
			ID:        h.ID,
			TaskID:    h.TaskID,
			ChangedBy: h.ChangedBy,
			Field:     h.Field,
			OldValue:  h.OldValue,
			NewValue:  h.NewValue,
			ChangedAt: h.ChangedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// ---------------- helpers ----------------

func toTaskResponse(t entity.Task) dto.TaskResponse {
	createdAt := ""
	updatedAt := ""
	if !t.CreatedAt.IsZero() {
		createdAt = t.CreatedAt.Format(time.RFC3339)
	}
	if !t.UpdatedAt.IsZero() {
		updatedAt = t.UpdatedAt.Format(time.RFC3339)
	}
	return dto.TaskResponse{
		ID:          t.ID,
		TeamID:      t.TeamID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		AssigneeID:  t.AssigneeID,
		CreatedBy:   t.CreatedBy,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

func buildPatchRich(req dto.UpdateTaskRequest) (repository.TaskPatch, bool, bool) {
	var p repository.TaskPatch
	changes := 0

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" || len(title) > 255 {
			return p, false, false
		}
		p.Title = &title
		changes++
	}
	if req.Description != nil {
		desc := *req.Description
		p.Description = &desc
		changes++
	}
	if req.Status != nil {
		if !req.Status.IsValid() {
			return p, false, false
		}
		p.Status = req.Status
		changes++
	}
	if req.AssigneeID != nil {
		// nil = снять исполнителя.
		if *req.AssigneeID == 0 {
			p.ClearAssignee = true
		} else {
			id := *req.AssigneeID
			p.AssigneeID = &id
		}
		changes++
	}
	return p, true, changes > 0
}

func buildHistoryEntries(
	taskID, actorID uint64,
	now time.Time,
	oldT *entity.Task, newT *entity.Task, patch repository.TaskPatch,
) []entity.TaskHistory {
	strPtr := func(s string) *string { return &s }
	out := make([]entity.TaskHistory, 0, 4)

	if patch.Title != nil && oldT.Title != newT.Title {
		out = append(out, entity.TaskHistory{
			TaskID:    taskID,
			ChangedBy: actorID,
			Field:     "title",
			OldValue:  strPtr(oldT.Title),
			NewValue:  strPtr(newT.Title),
			ChangedAt: now,
		})
	}
	if patch.Description != nil && (oldT.Description == nil || newT.Description == nil ||
		(oldT.Description != nil && newT.Description != nil && *oldT.Description != *newT.Description)) {
		out = append(out, entity.TaskHistory{
			TaskID:    taskID,
			ChangedBy: actorID,
			Field:     "description",
			OldValue:  oldT.Description,
			NewValue:  newT.Description,
			ChangedAt: now,
		})
	}
	if patch.Status != nil && oldT.Status != newT.Status {
		out = append(out, entity.TaskHistory{
			TaskID:    taskID,
			ChangedBy: actorID,
			Field:     "status",
			OldValue:  strPtr(string(oldT.Status)),
			NewValue:  strPtr(string(newT.Status)),
			ChangedAt: now,
		})
	}
	if patch.AssigneeID != nil || patch.ClearAssignee {
		var oldStr, newStr *string
		switch {
		case oldT.AssigneeID != nil:
			s := fmt.Sprintf("%d", *oldT.AssigneeID)
			oldStr = &s
		case oldT.AssigneeID == nil:
			oldStr = nil
		}
		switch {
		case newT.AssigneeID != nil:
			s := fmt.Sprintf("%d", *newT.AssigneeID)
			newStr = &s
		case newT.AssigneeID == nil:
			newStr = nil
		}
		out = append(out, entity.TaskHistory{
			TaskID:    taskID,
			ChangedBy: actorID,
			Field:     "assignee_id",
			OldValue:  oldStr,
			NewValue:  newStr,
			ChangedAt: now,
		})
	}
	return out
}
