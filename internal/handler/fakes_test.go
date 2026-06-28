package handler_test

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/repository"
	"github.com/example/go-project/internal/service"
)

type hFakeUserRepo struct {
	mu   sync.Mutex
	byID map[uint64]entity.User
	next uint64
}

func newHFakeUserRepo() *hFakeUserRepo {
	return &hFakeUserRepo{byID: map[uint64]entity.User{}}
}

func (r *hFakeUserRepo) Create(_ context.Context, u entity.User) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.byID {
		if x.Email == u.Email {
			return 0, repository.ErrAlreadyExists
		}
	}
	r.next++
	u.ID = r.next
	r.byID[u.ID] = u
	return u.ID, nil
}
func (r *hFakeUserRepo) FindByID(_ context.Context, id uint64) (*entity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &u, nil
}
func (r *hFakeUserRepo) FindByEmail(_ context.Context, email string) (*entity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, u := range r.byID {
		if u.Email == email {
			return &u, nil
		}
		_ = id
	}
	return nil, nil
}
func (r *hFakeUserRepo) Seed(email, name string) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	u := entity.User{ID: r.next, Email: email, Name: name, PasswordHash: "x",
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	r.byID[u.ID] = u
	return u.ID
}

type hFakeTeamRepo struct {
	mu      sync.Mutex
	teams   map[uint64]entity.Team
	members map[uint64]map[uint64]entity.TeamMember
	nextT   uint64
}

func newHFakeTeamRepo() *hFakeTeamRepo {
	return &hFakeTeamRepo{
		teams:   map[uint64]entity.Team{},
		members: map[uint64]map[uint64]entity.TeamMember{},
	}
}

func (r *hFakeTeamRepo) Create(_ context.Context, t entity.Team) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextT++
	t.ID = r.nextT
	r.teams[t.ID] = t
	return t.ID, nil
}
func (r *hFakeTeamRepo) FindByID(_ context.Context, id uint64) (*entity.Team, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.teams[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &t, nil
}
func (r *hFakeTeamRepo) ListByUser(_ context.Context, userID uint64) ([]repository.TeamWithRole, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []repository.TeamWithRole
	for _, m := range r.members {
		if member, ok := m[userID]; ok {
			t, exists := r.teams[member.TeamID]
			if exists {
				out = append(out, repository.TeamWithRole{Team: t, Role: member.Role})
			}
		}
	}
	return out, nil
}
func (r *hFakeTeamRepo) AddMember(_ context.Context, m entity.TeamMember) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.members[m.TeamID]; !ok {
		r.members[m.TeamID] = map[uint64]entity.TeamMember{}
	}
	if _, exists := r.members[m.TeamID][m.UserID]; exists {
		return repository.ErrAlreadyExists
	}
	r.members[m.TeamID][m.UserID] = m
	return nil
}
func (r *hFakeTeamRepo) GetMember(_ context.Context, teamID, userID uint64) (*entity.TeamMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tm, ok := r.members[teamID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	m, ok := tm[userID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &m, nil
}
func (r *hFakeTeamRepo) ListMembers(_ context.Context, teamID uint64) ([]entity.TeamMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tm, ok := r.members[teamID]
	if !ok {
		return nil, nil
	}
	out := make([]entity.TeamMember, 0, len(tm))
	for _, m := range tm {
		out = append(out, m)
	}
	return out, nil
}
func (r *hFakeTeamRepo) UpdateMemberRole(_ context.Context, teamID, userID uint64, role entity.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	tm, ok := r.members[teamID]
	if !ok {
		return repository.ErrNotFound
	}
	m, ok := tm[userID]
	if !ok {
		return repository.ErrNotFound
	}
	m.Role = role
	tm[userID] = m
	return nil
}
func (r *hFakeTeamRepo) RemoveMember(_ context.Context, teamID, userID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	tm, ok := r.members[teamID]
	if !ok {
		return repository.ErrNotFound
	}
	if _, ok := tm[userID]; !ok {
		return repository.ErrNotFound
	}
	delete(tm, userID)
	return nil
}
func (r *hFakeTeamRepo) UserBelongsToTeam(_ context.Context, teamID, userID uint64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tm, ok := r.members[teamID]
	if !ok {
		return false, nil
	}
	_, ok = tm[userID]
	return ok, nil
}
func (r *hFakeTeamRepo) SeedTeam(id uint64, name string, owner uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.teams[id] = entity.Team{ID: id, Name: name, CreatedBy: owner,
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if id > r.nextT {
		r.nextT = id
	}
}
func (r *hFakeTeamRepo) SeedMember(teamID, userID uint64, role entity.Role) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.members[teamID]; !ok {
		r.members[teamID] = map[uint64]entity.TeamMember{}
	}
	r.members[teamID][userID] = entity.TeamMember{
		UserID: userID, TeamID: teamID, Role: role, JoinedAt: time.Now(),
	}
}

type hFakeTaskRepo struct {
	mu    sync.Mutex
	tasks map[uint64]entity.Task
	next  uint64
}

func newHFakeTaskRepo() *hFakeTaskRepo {
	return &hFakeTaskRepo{tasks: map[uint64]entity.Task{}}
}

func (r *hFakeTaskRepo) Create(_ context.Context, t entity.Task) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	t.ID = r.next
	r.tasks[t.ID] = t
	return t.ID, nil
}
func (r *hFakeTaskRepo) FindByID(_ context.Context, id uint64) (*entity.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &t, nil
}
func (r *hFakeTaskRepo) List(_ context.Context, f dto.TaskFilter) ([]entity.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := hFilterTasksLocked(r.tasks, f)
	offset := f.Offset()
	if offset >= len(out) {
		return nil, nil
	}
	out = out[offset:]
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}
func (r *hFakeTaskRepo) Count(_ context.Context, f dto.TaskFilter) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(hFilterTasksLocked(r.tasks, f)), nil
}
func (r *hFakeTaskRepo) Update(ctx context.Context, id uint64, patch repository.TaskPatch) (*entity.Task, error) {
	return r.UpdateTx(ctx, nil, id, patch)
}
func (r *hFakeTaskRepo) UpdateTx(_ context.Context, _ service.TxExec, id uint64, patch repository.TaskPatch) (*entity.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok || t.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}
	if patch.Title != nil {
		t.Title = *patch.Title
	}
	if patch.Description != nil {
		t.Description = patch.Description
	}
	if patch.Status != nil {
		t.Status = *patch.Status
	}
	if patch.ClearAssignee {
		t.AssigneeID = nil
	} else if patch.AssigneeID != nil {
		t.AssigneeID = patch.AssigneeID
	}
	t.UpdatedAt = time.Now()
	r.tasks[id] = t
	return &t, nil
}
func (r *hFakeTaskRepo) SoftDelete(_ context.Context, id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok || t.DeletedAt != nil {
		return repository.ErrNotFound
	}
	now := time.Now()
	t.DeletedAt = &now
	r.tasks[id] = t
	return nil
}
func (r *hFakeTaskRepo) SeedTask(id uint64, t entity.Task) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t.ID = id
	r.tasks[id] = t
	if id > r.next {
		r.next = id
	}
}

func hFilterTasksLocked(tasks map[uint64]entity.Task, f dto.TaskFilter) []entity.Task {
	var out []entity.Task
	for _, t := range tasks {
		if t.DeletedAt != nil {
			continue
		}
		if f.TeamID != 0 && t.TeamID != f.TeamID {
			continue
		}
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		if f.AssigneeID != nil {
			if t.AssigneeID == nil || *t.AssigneeID != *f.AssigneeID {
				continue
			}
		}
		out = append(out, t)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].ID < out[i].ID {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

type hFakeHistoryRepo struct {
	mu       sync.Mutex
	byTaskID map[uint64][]entity.TaskHistory
	next     uint64
}

func newHFakeHistoryRepo() *hFakeHistoryRepo {
	return &hFakeHistoryRepo{byTaskID: map[uint64][]entity.TaskHistory{}}
}
func (r *hFakeHistoryRepo) Insert(_ context.Context, h entity.TaskHistory) error {
	return r.InsertTx(context.Background(), nil, h)
}
func (r *hFakeHistoryRepo) InsertTx(_ context.Context, _ service.TxExec, h entity.TaskHistory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	h.ID = r.next
	r.byTaskID[h.TaskID] = append(r.byTaskID[h.TaskID], h)
	return nil
}
func (r *hFakeHistoryRepo) ListByTask(_ context.Context, taskID uint64) ([]entity.TaskHistory, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows, ok := r.byTaskID[taskID]
	if !ok {
		return nil, nil
	}
	out := make([]entity.TaskHistory, len(rows))
	copy(out, rows)
	return out, nil
}

type hFakeCommentRepo struct {
	mu     sync.Mutex
	byTask map[uint64][]entity.TaskComment
	next   uint64
}

func newHFakeCommentRepo() *hFakeCommentRepo {
	return &hFakeCommentRepo{byTask: map[uint64][]entity.TaskComment{}}
}
func (r *hFakeCommentRepo) Create(_ context.Context, c entity.TaskComment) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	c.ID = r.next
	r.byTask[c.TaskID] = append(r.byTask[c.TaskID], c)
	return c.ID, nil
}
func (r *hFakeCommentRepo) InsertTx(_ context.Context, _ service.TxExec, c entity.TaskComment) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	c.ID = r.next
	r.byTask[c.TaskID] = append(r.byTask[c.TaskID], c)
	return c.ID, nil
}
func (r *hFakeCommentRepo) ListByTask(_ context.Context, taskID uint64) ([]entity.TaskComment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows, ok := r.byTask[taskID]
	if !ok {
		return nil, nil
	}
	out := make([]entity.TaskComment, len(rows))
	copy(out, rows)
	return out, nil
}

type hFakeStatsRepo struct {
	teamStats   []dto.TeamStatsResponse
	topCreators []dto.TopCreatorEntry
	orphanTasks []dto.OrphanTaskResponse
	calls       atomic.Uint64
}

func newHFakeStatsRepo() *hFakeStatsRepo { return &hFakeStatsRepo{} }
func (f *hFakeStatsRepo) TeamStatsLastWeek(_ context.Context) ([]dto.TeamStatsResponse, error) {
	f.calls.Add(1)
	return f.teamStats, nil
}
func (f *hFakeStatsRepo) TopCreatorsByTeam(_ context.Context, _, _ int) ([]dto.TopCreatorEntry, error) {
	f.calls.Add(1)
	return f.topCreators, nil
}
func (f *hFakeStatsRepo) OrphanTasks(_ context.Context) ([]dto.OrphanTaskResponse, error) {
	f.calls.Add(1)
	return f.orphanTasks, nil
}

type hFakeCache struct {
	kv map[string]string
}

func newHFakeCache() *hFakeCache { return &hFakeCache{kv: map[string]string{}} }
func (c *hFakeCache) Get(_ context.Context, key string) (string, error) {
	v, ok := c.kv[key]
	if !ok {
		return "", nil
	}
	return v, nil
}
func (c *hFakeCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	c.kv[key] = value
	return nil
}
func (c *hFakeCache) SetEX(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.Set(ctx, key, value, ttl)
}
func (c *hFakeCache) Del(_ context.Context, key string) error {
	delete(c.kv, key)
	return nil
}

type hFakeTransactor struct{}

func newHFakeTransactor() *hFakeTransactor { return &hFakeTransactor{} }
func (f *hFakeTransactor) WithinTx(_ context.Context, fn func(service.TxExec) error) error {
	return fn(nil)
}
