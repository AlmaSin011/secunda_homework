package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/example/go-project/internal/cache"
	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
)

// fakeCache — in-memory реализация cache.Cache для unit-тестов.
type fakeCache struct {
	kv map[string]string
}

func newFakeCache() *fakeCache { return &fakeCache{kv: map[string]string{}} }

func (c *fakeCache) Get(_ context.Context, key string) (string, error) {
	v, ok := c.kv[key]
	if !ok {
		return "", nil // промах = пусто (как в RedisCache.Get)
	}
	return v, nil
}
func (c *fakeCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	c.kv[key] = value
	return nil
}
func (c *fakeCache) SetEX(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.Set(ctx, key, value, ttl)
}
func (c *fakeCache) Del(_ context.Context, key string) error {
	delete(c.kv, key)
	return nil
}

// требуется контрактом cache.Cache:
var _ cache.Cache = (*fakeCache)(nil)

func TestTaskService_Create_OK(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	task, err := svc.Create(context.Background(), uid, dto.CreateTaskRequest{
		TeamID: 1, Title: "first task", Status: entity.TaskTodo,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.Title != "first task" {
		t.Fatalf("title: got %q", task.Title)
	}
	if task.CreatedBy != uid {
		t.Fatalf("createdBy: got %d want %d", task.CreatedBy, uid)
	}
}

func TestTaskService_Create_NotTeamMember(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	stranger := seedUser(t, users, "s@example.com", "S")
	teams.seedTeam(1, "T", owner)
	teams.seedMember(1, owner, entity.RoleOwner)

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	_, err := svc.Create(context.Background(), stranger, dto.CreateTaskRequest{
		TeamID: 1, Title: "x", Status: entity.TaskTodo,
	})
	if err != ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestTaskService_Create_Validation(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())

	cases := []dto.CreateTaskRequest{
		{TeamID: 1, Title: "", Status: entity.TaskTodo},
		{TeamID: 1, Title: "   ", Status: entity.TaskTodo},
		{TeamID: 1, Title: strings.Repeat("x", 256), Status: entity.TaskTodo},
		{TeamID: 1, Title: "ok", Status: entity.TaskStatus("nope")},
		{TeamID: 0, Title: "ok", Status: entity.TaskTodo},
	}
	for i, req := range cases {
		if _, err := svc.Create(context.Background(), uid, req); err != ErrValidation {
			t.Fatalf("case %d: want ErrValidation, got %v", i, err)
		}
	}
}

func TestTaskService_List_OK(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "a", Status: entity.TaskTodo, CreatedBy: uid})
	tasks.seedTask(2, entity.Task{TeamID: 1, Title: "b", Status: entity.TaskDone, CreatedBy: uid})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	out, err := svc.List(context.Background(), uid, dto.TaskFilter{TeamID: 1, Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("len: got %d want 2", len(out.Items))
	}
	if out.Meta.Total != 2 {
		t.Fatalf("total: got %d want 2", out.Meta.Total)
	}
}

func TestTaskService_List_FilterByStatus(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "a", Status: entity.TaskTodo, CreatedBy: uid})
	tasks.seedTask(2, entity.Task{TeamID: 1, Title: "b", Status: entity.TaskDone, CreatedBy: uid})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	out, err := svc.List(context.Background(), uid, dto.TaskFilter{
		TeamID: 1, Status: entity.TaskDone, Page: 1, Limit: 10,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out.Items) != 1 || out.Items[0].Status != entity.TaskDone {
		t.Fatalf("filter failed: %+v", out)
	}
}

func TestTaskService_List_ForbiddenIfNotMember(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	stranger := seedUser(t, users, "s@example.com", "S")
	teams.seedTeam(1, "T", owner)
	teams.seedMember(1, owner, entity.RoleOwner)

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	_, err := svc.List(context.Background(), stranger, dto.TaskFilter{TeamID: 1, Page: 1, Limit: 10})
	if err != ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestTaskService_List_NoTeamID(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	_, err := svc.List(context.Background(), uid, dto.TaskFilter{Page: 1, Limit: 10})
	if err != ErrValidation {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestTaskService_List_BadStatus(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	_, err := svc.List(context.Background(), uid, dto.TaskFilter{
		TeamID: 1, Status: entity.TaskStatus("xxx"), Page: 1, Limit: 10,
	})
	if err != ErrValidation {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestTaskService_Get_ForbiddenIfNotMember(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	stranger := seedUser(t, users, "s@example.com", "S")
	teams.seedTeam(1, "T", owner)
	teams.seedMember(1, owner, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "a", Status: entity.TaskTodo, CreatedBy: owner})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	_, err := svc.Get(context.Background(), stranger, 1)
	if err != ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestTaskService_Update_GeneratesHistory(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	tx := newFakeTransactor()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{
		TeamID: 1, Title: "old", Status: entity.TaskTodo, CreatedBy: uid,
	})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), tx)
	newTitle := "new"
	newStatus := entity.TaskInProgress
	updated, err := svc.Update(context.Background(), uid, 1, dto.UpdateTaskRequest{
		Title: &newTitle, Status: &newStatus,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != "new" || updated.Status != entity.TaskInProgress {
		t.Fatalf("updated: %+v", updated)
	}
	// Транзакция была выполнена.
	if tx.calls.Load() != 1 {
		t.Fatalf("transactor calls: got %d want 1", tx.calls.Load())
	}
	// История содержит две записи (title и status).
	entries := history.Entries()
	if len(entries) != 2 {
		t.Fatalf("history entries: got %d want 2", len(entries))
	}
	fields := map[string]bool{}
	for _, e := range entries {
		fields[e.Field] = true
	}
	if !fields["title"] || !fields["status"] {
		t.Fatalf("history fields: %+v", fields)
	}
}

func TestTaskService_Update_ClearAssignee(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	assignee := uint64(42)
	tasks.seedTask(1, entity.Task{
		TeamID: 1, Title: "x", Status: entity.TaskTodo,
		CreatedBy: uid, AssigneeID: &assignee,
	})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	zero := uint64(0)
	updated, err := svc.Update(context.Background(), uid, 1, dto.UpdateTaskRequest{
		AssigneeID: &zero,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.AssigneeID != nil {
		t.Fatalf("assignee: want nil, got %v", *updated.AssigneeID)
	}
}

func TestTaskService_Update_NoFields(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	// Пустой Update эквивалентен Get.
	got, err := svc.Update(context.Background(), uid, 1, dto.UpdateTaskRequest{})
	if err != nil {
		t.Fatalf("update no fields: %v", err)
	}
	if got.Title != "x" {
		t.Fatalf("title: got %q", got.Title)
	}
}

func TestTaskService_Update_ValidationError(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	bad := entity.TaskStatus("nope")
	_, err := svc.Update(context.Background(), uid, 1, dto.UpdateTaskRequest{Status: &bad})
	if err != ErrValidation {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestTaskService_Update_NotFound(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	title := "x"
	_, err := svc.Update(context.Background(), uid, 999, dto.UpdateTaskRequest{Title: &title})
	if err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestTaskService_Delete_Forbidden(t *testing.T) {
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
	if err := svc.Delete(context.Background(), stranger, 1); err != ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestTaskService_History_OK(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	history := newFakeHistoryRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})
	_ = history.InsertTx(context.Background(), nil, entity.TaskHistory{
		TaskID: 1, ChangedBy: uid, Field: "title", ChangedAt: now(), NewValue: ptr("x"),
	})

	svc := NewTaskService(tasks, history, teams, newFakeCache(), newFakeTransactor())
	rows, err := svc.History(context.Background(), uid, 1)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len: got %d want 1", len(rows))
	}
}

func TestTaskService_History_ForbiddenIfNotMember(t *testing.T) {
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
	_, err := svc.History(context.Background(), stranger, 1)
	if err != ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}
