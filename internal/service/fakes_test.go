package service

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/repository"
)

type fakeUserRepo struct {
	mu    sync.Mutex
	byID  map[uint64]entity.User
	byKey map[string]uint64
	next  uint64
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byID:  map[uint64]entity.User{},
		byKey: map[string]uint64{},
	}
}

func (f *fakeUserRepo) Create(_ context.Context, u entity.User) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.ToLower(strings.TrimSpace(u.Email))
	if _, exists := f.byKey[key]; exists {
		return 0, repository.ErrAlreadyExists
	}
	f.next++
	u.ID = f.next
	f.byID[u.ID] = u
	f.byKey[key] = u.ID
	return u.ID, nil
}

func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (*entity.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byKey[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return nil, nil
	}
	u := f.byID[id]
	return &u, nil
}

func (f *fakeUserRepo) FindByID(_ context.Context, id uint64) (*entity.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &u, nil
}

type fakeTeamRepo struct {
	mu      sync.Mutex
	teams   map[uint64]entity.Team
	members map[uint64]map[uint64]entity.TeamMember // teamID -> userID -> member
	nextT   uint64
}

func newFakeTeamRepo() *fakeTeamRepo {
	return &fakeTeamRepo{
		teams:   map[uint64]entity.Team{},
		members: map[uint64]map[uint64]entity.TeamMember{},
	}
}

func (r *fakeTeamRepo) Create(_ context.Context, t entity.Team) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextT++
	t.ID = r.nextT
	r.teams[t.ID] = t
	return t.ID, nil
}

func (r *fakeTeamRepo) FindByID(_ context.Context, id uint64) (*entity.Team, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.teams[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &t, nil
}

func (r *fakeTeamRepo) ListByUser(_ context.Context, userID uint64) ([]repository.TeamWithRole, error) {
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

func (r *fakeTeamRepo) AddMember(_ context.Context, m entity.TeamMember) error {
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

func (r *fakeTeamRepo) GetMember(_ context.Context, teamID, userID uint64) (*entity.TeamMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	teamMembers, ok := r.members[teamID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	m, ok := teamMembers[userID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &m, nil
}

func (r *fakeTeamRepo) ListMembers(_ context.Context, teamID uint64) ([]entity.TeamMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	teamMembers, ok := r.members[teamID]
	if !ok {
		return nil, nil
	}
	out := make([]entity.TeamMember, 0, len(teamMembers))
	for _, m := range teamMembers {
		out = append(out, m)
	}
	return out, nil
}

func (r *fakeTeamRepo) UpdateMemberRole(_ context.Context, teamID, userID uint64, role entity.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	teamMembers, ok := r.members[teamID]
	if !ok {
		return repository.ErrNotFound
	}
	m, ok := teamMembers[userID]
	if !ok {
		return repository.ErrNotFound
	}
	m.Role = role
	teamMembers[userID] = m
	return nil
}

func (r *fakeTeamRepo) RemoveMember(_ context.Context, teamID, userID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	teamMembers, ok := r.members[teamID]
	if !ok {
		return repository.ErrNotFound
	}
	if _, ok := teamMembers[userID]; !ok {
		return repository.ErrNotFound
	}
	delete(teamMembers, userID)
	return nil
}

func (r *fakeTeamRepo) UserBelongsToTeam(_ context.Context, teamID, userID uint64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	teamMembers, ok := r.members[teamID]
	if !ok {
		return false, nil
	}
	_, ok = teamMembers[userID]
	return ok, nil
}

func (r *fakeTeamRepo) seedTeam(id uint64, name string, createdBy uint64) entity.Team {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := entity.Team{
		ID:        id,
		Name:      name,
		CreatedBy: createdBy,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	r.teams[id] = t
	if id > r.nextT {
		r.nextT = id
	}
	return t
}

func (r *fakeTeamRepo) seedMember(teamID, userID uint64, role entity.Role) entity.TeamMember {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.members[teamID]; !ok {
		r.members[teamID] = map[uint64]entity.TeamMember{}
	}
	m := entity.TeamMember{
		UserID:   userID,
		TeamID:   teamID,
		Role:     role,
		JoinedAt: time.Now(),
	}
	r.members[teamID][userID] = m
	return m
}

type fakeTaskRepo struct {
	mu    sync.Mutex
	tasks map[uint64]entity.Task
	next  uint64
}

func newFakeTaskRepo() *fakeTaskRepo {
	return &fakeTaskRepo{tasks: map[uint64]entity.Task{}}
}

func (r *fakeTaskRepo) Create(_ context.Context, t entity.Task) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	t.ID = r.next
	r.tasks[t.ID] = t
	return t.ID, nil
}

func (r *fakeTaskRepo) FindByID(_ context.Context, id uint64) (*entity.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &t, nil
}

func (r *fakeTaskRepo) List(_ context.Context, f dto.TaskFilter) ([]entity.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := filterTasksLocked(r.tasks, f)
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

func (r *fakeTaskRepo) Count(_ context.Context, f dto.TaskFilter) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(filterTasksLocked(r.tasks, f)), nil
}

func (r *fakeTaskRepo) Update(ctx context.Context, id uint64, patch repository.TaskPatch) (*entity.Task, error) {
	return r.UpdateTx(ctx, nil, id, patch)
}

func (r *fakeTaskRepo) UpdateTx(_ context.Context, _ TxExec, id uint64, patch repository.TaskPatch) (*entity.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if t.DeletedAt != nil {
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

func (r *fakeTaskRepo) SoftDelete(_ context.Context, id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return repository.ErrNotFound
	}
	if t.DeletedAt != nil {
		return repository.ErrNotFound
	}
	now := time.Now()
	t.DeletedAt = &now
	r.tasks[id] = t
	return nil
}

func (r *fakeTaskRepo) seedTask(id uint64, t entity.Task) entity.Task {
	r.mu.Lock()
	defer r.mu.Unlock()
	t.ID = id
	r.tasks[id] = t
	if id > r.next {
		r.next = id
	}
	return t
}

func filterTasksLocked(tasks map[uint64]entity.Task, f dto.TaskFilter) []entity.Task {
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
	// Сортируем по ID для соответствия "ORDER BY id".
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].ID < out[i].ID {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

type fakeHistoryRepo struct {
	mu       sync.Mutex
	entries  []entity.TaskHistory
	byTaskID map[uint64][]entity.TaskHistory
	next     uint64
}

func newFakeHistoryRepo() *fakeHistoryRepo {
	return &fakeHistoryRepo{byTaskID: map[uint64][]entity.TaskHistory{}}
}

func (r *fakeHistoryRepo) Insert(_ context.Context, h entity.TaskHistory) error {
	return r.InsertTx(context.Background(), nil, h)
}

func (r *fakeHistoryRepo) InsertTx(_ context.Context, _ TxExec, h entity.TaskHistory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	h.ID = r.next
	r.entries = append(r.entries, h)
	r.byTaskID[h.TaskID] = append(r.byTaskID[h.TaskID], h)
	return nil
}

func (r *fakeHistoryRepo) ListByTask(_ context.Context, taskID uint64) ([]entity.TaskHistory, error) {
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

func (r *fakeHistoryRepo) Entries() []entity.TaskHistory {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]entity.TaskHistory, len(r.entries))
	copy(out, r.entries)
	return out
}

type fakeCommentRepo struct {
	mu     sync.Mutex
	byTask map[uint64][]entity.TaskComment
	next   uint64
}

func newFakeCommentRepo() *fakeCommentRepo {
	return &fakeCommentRepo{byTask: map[uint64][]entity.TaskComment{}}
}

func (r *fakeCommentRepo) Create(_ context.Context, c entity.TaskComment) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	c.ID = r.next
	r.byTask[c.TaskID] = append(r.byTask[c.TaskID], c)
	return c.ID, nil
}

// InsertTx — Tx-аналог Create: при наличии транзакции эмулируем коммит сразу,
// но никогда не откатываем (для unit-тестов достаточно).
func (r *fakeCommentRepo) InsertTx(_ context.Context, _ TxExec, c entity.TaskComment) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	c.ID = r.next
	r.byTask[c.TaskID] = append(r.byTask[c.TaskID], c)
	return c.ID, nil
}

func (r *fakeCommentRepo) ListByTask(_ context.Context, taskID uint64) ([]entity.TaskComment, error) {
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

// ---------------- fakeStatsRepo ----------------

type fakeStatsRepo struct {
	mu sync.Mutex

	teamStats   []dto.TeamStatsResponse
	topCreators []dto.TopCreatorEntry
	orphanTasks []dto.OrphanTaskResponse

	teamStatsCalls atomic.Uint64
	topCalls       atomic.Uint64
	orphanCalls    atomic.Uint64
	lastSinceDays  atomic.Int64
	lastLimit      atomic.Int64
	err            error
}

func newFakeStatsRepo() *fakeStatsRepo {
	return &fakeStatsRepo{}
}

func (f *fakeStatsRepo) TeamStatsLastWeek(_ context.Context) ([]dto.TeamStatsResponse, error) {
	f.teamStatsCalls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	return f.teamStats, nil
}

func (f *fakeStatsRepo) TopCreatorsByTeam(_ context.Context, sinceDays, limit int) ([]dto.TopCreatorEntry, error) {
	f.topCalls.Add(1)
	f.lastSinceDays.Store(int64(sinceDays))
	f.lastLimit.Store(int64(limit))
	if f.err != nil {
		return nil, f.err
	}
	return f.topCreators, nil
}

func (f *fakeStatsRepo) OrphanTasks(_ context.Context) ([]dto.OrphanTaskResponse, error) {
	f.orphanCalls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	return f.orphanTasks, nil
}

type fakeTransactor struct {
	mu    sync.Mutex
	calls atomic.Uint64
	err   error
}

func newFakeTransactor() *fakeTransactor {
	return &fakeTransactor{}
}

func (f *fakeTransactor) WithinTx(ctx context.Context, fn func(TxExec) error) error {
	f.calls.Add(1)
	if f.err != nil {
		return f.err
	}
	return fn(nil)
}

var (
	_ UserRepository    = (*fakeUserRepo)(nil)
	_ TeamRepository    = (*fakeTeamRepo)(nil)
	_ TaskRepository    = (*fakeTaskRepo)(nil)
	_ HistoryRepository = (*fakeHistoryRepo)(nil)
	_ CommentRepository = (*fakeCommentRepo)(nil)
	_ StatsRepository   = (*fakeStatsRepo)(nil)
	_ Transactor        = (*fakeTransactor)(nil)
)
