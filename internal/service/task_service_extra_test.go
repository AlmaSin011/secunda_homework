package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/repository"
)

func TestTaskService_List_CacheHit(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "a", Status: entity.TaskTodo, CreatedBy: uid})

	c := newFakeCache()
	svc := NewTaskService(tasks, history, teams, c, newFakeTransactor())

	filter := dto.TaskFilter{TeamID: 1, Page: 1, Limit: 10}
	// Первый вызов — cache miss, читаем из БД и сохраняем в кеш.
	first, err := svc.List(context.Background(), uid, filter)
	if err != nil {
		t.Fatalf("first list: %v", err)
	}
	if len(first.Items) != 1 {
		t.Fatalf("first len: got %d", len(first.Items))
	}

	// Удаляем задачу из репозитория — второй вызов должен вернуть данные из кеша.
	now := time.Now()
	tasks.tasks[1] = entity.Task{
		ID: 1, TeamID: 99, Title: "gone", Status: entity.TaskTodo,
		CreatedBy: uid, DeletedAt: &now,
	}

	second, err := svc.List(context.Background(), uid, filter)
	if err != nil {
		t.Fatalf("second list: %v", err)
	}
	if len(second.Items) != 1 || second.Items[0].Title != "a" {
		t.Fatalf("expected cache hit, got: %+v", second)
	}
}

func TestTaskService_List_CacheNil(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "a", Status: entity.TaskTodo, CreatedBy: uid})

	svc := NewTaskService(tasks, history, teams, nil, newFakeTransactor())
	out, err := svc.List(context.Background(), uid, dto.TaskFilter{TeamID: 1, Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("list no cache: %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("len: got %d want 1", len(out.Items))
	}
}

func TestTaskService_Get_NotFound(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())

	_, err := svc.Get(context.Background(), uid, 999)
	if err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestTaskService_Get_OK(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	got, err := svc.Get(context.Background(), uid, 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "x" {
		t.Fatalf("title: %q", got.Title)
	}
}

// TestTaskService_Delete_OK — успешное удаление.
func TestTaskService_Delete_OK(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	if err := svc.Delete(context.Background(), uid, 1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	rows, _ := tasks.List(context.Background(), dto.TaskFilter{TeamID: 1, Page: 1, Limit: 10})
	if len(rows) != 0 {
		t.Fatalf("expected empty list after soft delete, got %d", len(rows))
	}
}

func TestTaskService_Delete_NotFound(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())

	if err := svc.Delete(context.Background(), uid, 999); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestTaskService_Update_SetAssignee(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	assignee := uint64(42)
	updated, err := svc.Update(context.Background(), uid, 1, dto.UpdateTaskRequest{
		AssigneeID: &assignee,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.AssigneeID == nil || *updated.AssigneeID != 42 {
		t.Fatalf("assignee: %v", updated.AssigneeID)
	}
}

func TestTaskService_Update_Description(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{
		TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid,
	})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	desc := "new description"
	updated, err := svc.Update(context.Background(), uid, 1, dto.UpdateTaskRequest{
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Description == nil || *updated.Description != "new description" {
		t.Fatalf("description: %v", updated.Description)
	}
	if len(history.Entries()) != 1 || history.Entries()[0].Field != "description" {
		t.Fatalf("history entries: %+v", history.Entries())
	}
}

func TestTaskService_Update_TxFails(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "old", Status: entity.TaskTodo, CreatedBy: uid})

	tx := newFakeTransactor()
	tx.err = errors.New("tx failed")
	svc := NewTaskService(tasks, history, teams, newFakeCache(), tx)

	newTitle := "new"
	_, err := svc.Update(context.Background(), uid, 1, dto.UpdateTaskRequest{Title: &newTitle})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	// История не должна была записаться.
	if len(history.Entries()) != 0 {
		t.Fatalf("history should be empty, got %d", len(history.Entries()))
	}
}

func TestTaskService_History_NotFound(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())

	_, err := svc.History(context.Background(), uid, 999)
	if err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestTaskService_History_RepoError(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	_ = newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	bad := &errHistoryRepo{err: errors.New("db down")}

	svc := NewTaskService(tasks, bad, teams, newFakeCache(), newFakeTransactor())
	_, err := svc.History(context.Background(), uid, 1)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestTaskService_List_RepoError(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	bad := &errTaskRepo{err: errors.New("db down")}
	svc.tasks = bad
	_, err := svc.List(context.Background(), uid, dto.TaskFilter{TeamID: 1, Page: 1, Limit: 10})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestTaskService_Update_BadTitle(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	empty := "   "
	_, err := svc.Update(context.Background(), uid, 1, dto.UpdateTaskRequest{Title: &empty})
	if err != ErrValidation {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestTaskService_Update_LongTitle(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	long := ""
	for i := 0; i < 300; i++ {
		long += "a"
	}
	_, err := svc.Update(context.Background(), uid, 1, dto.UpdateTaskRequest{Title: &long})
	if err != ErrValidation {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestTaskService_Update_NotTeamMember(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	stranger := seedUser(t, users, "s@example.com", "S")
	teams.seedTeam(1, "T", owner)
	teams.seedMember(1, owner, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: owner})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	newTitle := "new"
	_, err := svc.Update(context.Background(), stranger, 1, dto.UpdateTaskRequest{Title: &newTitle})
	if err != ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestTaskService_Create_NoTeamID(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())

	_, err := svc.Create(context.Background(), uid, dto.CreateTaskRequest{
		Title: "x", Status: entity.TaskTodo,
	})
	if err != ErrValidation {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestTaskService_Get_RepoError(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	_ = newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)

	bad := &errTaskRepo{err: errors.New("db down")}
	svc := NewTaskService(bad, history, teams, newFakeCache(), newFakeTransactor())
	_, err := svc.Get(context.Background(), uid, 1)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestTaskService_Delete_RepoError(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	bad := &errTaskRepo{err: errors.New("db down"), tasks: tasks}
	svc := NewTaskService(bad, history, teams, newFakeCache(), newFakeTransactor())
	if err := svc.Delete(context.Background(), uid, 1); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestTaskService_Create_RepoError(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	_ = newFakeTaskRepo() //nolint:unused // seeds не нужны, т.к. tasks-repo подменён
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)

	bad := &errTaskRepo{err: errors.New("db down")}
	svc := NewTaskService(bad, history, teams, newFakeCache(), newFakeTransactor())
	_, err := svc.Create(context.Background(), uid, dto.CreateTaskRequest{
		TeamID: 1, Title: "x", Status: entity.TaskTodo,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

type errTaskRepo struct {
	err   error
	tasks *fakeTaskRepo
}

func (r *errTaskRepo) Create(_ context.Context, _ entity.Task) (uint64, error) { return 0, r.err }
func (r *errTaskRepo) FindByID(_ context.Context, _ uint64) (*entity.Task, error) {
	return nil, r.err
}
func (r *errTaskRepo) List(_ context.Context, _ dto.TaskFilter) ([]entity.Task, error) {
	return nil, r.err
}
func (r *errTaskRepo) Count(_ context.Context, _ dto.TaskFilter) (int, error) {
	return 0, r.err
}
func (r *errTaskRepo) Update(_ context.Context, _ uint64, _ repository.TaskPatch) (*entity.Task, error) {
	return nil, r.err
}
func (r *errTaskRepo) UpdateTx(_ context.Context, _ TxExec, _ uint64, _ repository.TaskPatch) (*entity.Task, error) {
	return nil, r.err
}
func (r *errTaskRepo) SoftDelete(_ context.Context, _ uint64) error { return r.err }

// прокси с ошибкой.
type errHistoryRepo struct {
	err error
}

func (r *errHistoryRepo) Insert(_ context.Context, _ entity.TaskHistory) error { return r.err }
func (r *errHistoryRepo) InsertTx(_ context.Context, _ TxExec, _ entity.TaskHistory) error {
	return r.err
}
func (r *errHistoryRepo) ListByTask(_ context.Context, _ uint64) ([]entity.TaskHistory, error) {
	return nil, r.err
}

func TestToTaskResponse_ZeroTimestamps(t *testing.T) {
	task := entity.Task{
		ID:     1,
		TeamID: 1,
		Title:  "x",
		Status: entity.TaskTodo,
		// CreatedAt и UpdatedAt = zero time
	}
	resp := toTaskResponse(task)
	if resp.CreatedAt != "" {
		t.Fatalf("CreatedAt: got %q want empty", resp.CreatedAt)
	}
	if resp.UpdatedAt != "" {
		t.Fatalf("UpdatedAt: got %q want empty", resp.UpdatedAt)
	}
}

func TestTaskService_List_CacheBroken(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "a", Status: entity.TaskTodo, CreatedBy: uid})

	c := newFakeCache()
	//  битый JSON в кеш для team=1, page=1
	c.kv["task:list:team:1:status:any:page:1"] = "{not-json"

	svc := NewTaskService(tasks, history, teams, c, newFakeTransactor())
	out, err := svc.List(context.Background(), uid, dto.TaskFilter{TeamID: 1, Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("len: got %d want 1 (broken cache should miss)", len(out.Items))
	}
	if _, ok := c.kv["task:list:team:1:status:any:page:1"]; !ok {
		t.Fatalf("cache key missing after Store")
	}
}
