//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/example/go-project/internal/cache"
	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/repository"
	"github.com/example/go-project/internal/service"
)

type memCache struct {
	mu   sync.Mutex
	data map[string]string
}

func newMemCache() *memCache { return &memCache{data: map[string]string{}} }

func (m *memCache) Get(_ context.Context, k string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[k]
	if !ok {
		return "", nil
	}
	return v, nil
}

func (m *memCache) Set(_ context.Context, k, v string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[k] = v
	return nil
}
func (m *memCache) SetEX(ctx context.Context, k, v string, ttl time.Duration) error {
	return m.Set(ctx, k, v, ttl)
}
func (m *memCache) Del(_ context.Context, k string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, k)
	return nil
}

func (m *memCache) has(k string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.data[k]
	return ok
}

type serviceHarness struct {
	userRepo  repository.UserRepository
	teamRepo  repository.TeamRepository
	taskRepo  repository.TaskRepository
	histRepo  repository.HistoryRepository
	commRepo  repository.CommentRepository
	statsRepo repository.StatsRepository
	cache     cache.Cache
	tx        service.Transactor
	teamSvc   *service.TeamService
	taskSvc   *service.TaskService
	commSvc   *service.CommentService
	statsSvc  *service.StatsService
}

func openServiceHarness(t *testing.T, withCache bool) *serviceHarness {
	t.Helper()
	db := mustOpen(t)
	t.Cleanup(func() { _ = db.Close() })
	cleanup(t, db)

	userRepo := repository.NewUserRepository(db)
	teamRepo := repository.NewTeamRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	histRepo := repository.NewHistoryRepository(db)
	commRepo := repository.NewCommentRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	var c cache.Cache
	if withCache {
		c = newMemCache()
	}
	tx := service.NewSQLXTransactor(db)

	h := &serviceHarness{
		userRepo:  userRepo,
		teamRepo:  teamRepo,
		taskRepo:  taskRepo,
		histRepo:  histRepo,
		commRepo:  commRepo,
		statsRepo: statsRepo,
		cache:     c,
		tx:        tx,
	}
	h.teamSvc = service.NewTeamService(teamRepo, userRepo, tx)
	h.taskSvc = service.NewTaskService(taskRepo, histRepo, teamRepo, c, tx)
	h.commSvc = service.NewCommentService(commRepo, taskRepo, teamRepo, tx)
	h.statsSvc = service.NewStatsService(statsRepo)
	return h
}

func newUser(t *testing.T, h *serviceHarness, email string) uint64 {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	id, err := h.userRepo.Create(context.Background(), entity.User{
		Email:        email,
		PasswordHash: "h",
		Name:         email,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return id
}

func newTeam(t *testing.T, h *serviceHarness, owner uint64, name string) uint64 {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	id, err := h.teamRepo.Create(context.Background(), entity.Team{
		Name:      name,
		CreatedBy: owner,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := h.teamRepo.AddMember(context.Background(), entity.TeamMember{
		UserID: owner, TeamID: id, Role: entity.RoleOwner, JoinedAt: now,
	}); err != nil {
		t.Fatalf("add owner member: %v", err)
	}
	return id
}

func addMember(t *testing.T, h *serviceHarness, teamID, userID uint64, role entity.Role) {
	t.Helper()
	if err := h.teamRepo.AddMember(context.Background(), entity.TeamMember{
		UserID: userID, TeamID: teamID, Role: role, JoinedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("add member: %v", err)
	}
}

func TestService_TaskUpdateInTransaction(t *testing.T) {
	skipIfNoDB(t)
	h := openServiceHarness(t, false)
	ctx := context.Background()

	owner := newUser(t, h, fmt.Sprintf("owner-svc-%d@example.com", time.Now().UnixNano()))
	member := newUser(t, h, fmt.Sprintf("member-svc-%d@example.com", time.Now().UnixNano()))
	teamID := newTeam(t, h, owner, "Svc Team")
	addMember(t, h, teamID, member, entity.RoleMember)

	// Создаём задачу через service.
	desc := "Initial"
	created, err := h.taskSvc.Create(ctx, owner, dto.CreateTaskRequest{
		TeamID:      teamID,
		Title:       "Atomic task",
		Description: &desc,
		Status:      entity.TaskTodo,
		AssigneeID:  &owner,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected non-zero task id")
	}

	// 1. Успешный upd с двумя изменёнными полями — должно появиться
	newTitle := "Atomic task v2"
	newStatus := entity.TaskInProgress
	upd, err := h.taskSvc.Update(ctx, member, created.ID, dto.UpdateTaskRequest{
		Title:  &newTitle,
		Status: &newStatus,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if upd.Title != newTitle {
		t.Errorf("title not updated: %q", upd.Title)
	}
	if upd.Status != newStatus {
		t.Errorf("status not updated: %q", upd.Status)
	}

	histRows, err := h.histRepo.ListByTask(ctx, created.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(histRows) != 2 {
		t.Fatalf("expected 2 history entries, got %d: %+v", len(histRows), histRows)
	}
	fields := map[string]bool{}
	for _, row := range histRows {
		fields[row.Field] = true
	}
	if !fields["title"] || !fields["status"] {
		t.Errorf("history missing fields: got %v", fields)
	}

	// 2. Update "теми же значениями" по всем полям → пустой патч → Get,
	sameStatus := upd.Status
	sameTitle := upd.Title
	if _, err := h.taskSvc.Update(ctx, member, created.ID, dto.UpdateTaskRequest{
		Title:  &sameTitle,
		Status: &sameStatus,
	}); err != nil {
		t.Fatalf("second Update: %v", err)
	}
	histRows2, _ := h.histRepo.ListByTask(ctx, created.ID)
	if len(histRows2) != len(histRows) {
		t.Errorf("expected no new history entries on idempotent update: was=%d now=%d",
			len(histRows), len(histRows2))
	}

	// 3. Снятие assignee.
	zeroID := uint64(0)
	if _, err := h.taskSvc.Update(ctx, owner, created.ID, dto.UpdateTaskRequest{
		AssigneeID: &zeroID,
	}); err != nil {
		t.Fatalf("clear assignee: %v", err)
	}
	got, _ := h.taskRepo.FindByID(ctx, created.ID)
	if got.AssigneeID != nil {
		t.Errorf("expected nil assignee, got %v", got.AssigneeID)
	}

	// 4. Update несуществующей задачи - ErrNotFound.
	if _, err := h.taskSvc.Update(ctx, owner, 999_999, dto.UpdateTaskRequest{
		Status: &newStatus,
	}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing task, got %v", err)
	}

	// 5. Update задачи не-членом команды → ErrForbidden.
	stranger := newUser(t, h, fmt.Sprintf("stranger-%d@example.com", time.Now().UnixNano()))
	if _, err := h.taskSvc.Update(ctx, stranger, created.ID, dto.UpdateTaskRequest{
		Title: &newTitle,
	}); !errors.Is(err, service.ErrForbidden) {
		t.Errorf("expected ErrForbidden for stranger, got %v", err)
	}
}

func TestService_TeamInvite(t *testing.T) {
	skipIfNoDB(t)
	h := openServiceHarness(t, false)
	ctx := context.Background()

	owner := newUser(t, h, fmt.Sprintf("owner-inv-%d@example.com", time.Now().UnixNano()))
	bob := newUser(t, h, fmt.Sprintf("bob-inv-%d@example.com", time.Now().UnixNano()))
	teamID := newTeam(t, h, owner, "Invite Team")

	// Owner приглашает пользака.
	if err := h.teamSvc.Invite(ctx, owner, teamID, dto.InviteRequest{
		UserID: bob, Role: entity.RoleMember,
	}); err != nil {
		t.Fatalf("invite: %v", err)
	}

	// Пользак теперь виден в списке.
	members, err := h.teamSvc.ListMembers(ctx, owner, teamID)
	if err != nil || len(members) != 2 {
		t.Errorf("expected 2 members, got %d err=%v", len(members), err)
	}

	if err := h.teamSvc.Invite(ctx, owner, teamID, dto.InviteRequest{
		UserID: bob, Role: entity.RoleMember,
	}); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists on duplicate invite, got %v", err)
	}

	carol := newUser(t, h, fmt.Sprintf("carol-inv-%d@example.com", time.Now().UnixNano()))
	if err := h.teamSvc.Invite(ctx, bob, teamID, dto.InviteRequest{
		UserID: carol, Role: entity.RoleMember,
	}); !errors.Is(err, service.ErrForbidden) {
		t.Errorf("expected ErrForbidden for member invite, got %v", err)
	}

	if err := h.teamSvc.Invite(ctx, owner, teamID, dto.InviteRequest{
		UserID: 999_999, Role: entity.RoleMember,
	}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing user, got %v", err)
	}
}

func TestService_CommentCreateAndList(t *testing.T) {
	skipIfNoDB(t)
	h := openServiceHarness(t, false)
	ctx := context.Background()

	owner := newUser(t, h, fmt.Sprintf("cmt-owner-%d@example.com", time.Now().UnixNano()))
	teamID := newTeam(t, h, owner, "Cmt Team")
	desc := "task"
	created, err := h.taskSvc.Create(ctx, owner, dto.CreateTaskRequest{
		TeamID: teamID, Title: "Cmt task", Description: &desc, Status: entity.TaskTodo,
	})
	if err != nil {
		t.Fatalf("Create task: %v", err)
	}

	for _, body := range []string{"first", "second", "third"} {
		id, err := h.commSvc.Create(ctx, owner, created.ID, dto.CreateCommentRequest{Body: body})
		if err != nil {
			t.Fatalf("create comment %q: %v", body, err)
		}
		if id == 0 {
			t.Fatal("expected non-zero comment id")
		}
		// небольшая задержка чтобы created_at отличались
		time.Sleep(2 * time.Millisecond)
	}

	list, err := h.commSvc.List(ctx, owner, created.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(list))
	}
	want := []string{"first", "second", "third"}
	for i, c := range list {
		if c.Body != want[i] {
			t.Errorf("comment[%d] body=%q want %q", i, c.Body, want[i])
		}
	}

	stranger := newUser(t, h, fmt.Sprintf("cmt-str-%d@example.com", time.Now().UnixNano()))
	if _, err := h.commSvc.Create(ctx, stranger, created.ID, dto.CreateCommentRequest{Body: "x"}); !errors.Is(err, service.ErrForbidden) {
		t.Errorf("expected ErrForbidden for stranger, got %v", err)
	}

	if _, err := h.commSvc.Create(ctx, owner, created.ID, dto.CreateCommentRequest{Body: "   "}); !errors.Is(err, service.ErrValidation) {
		t.Errorf("expected ErrValidation for empty body, got %v", err)
	}

	if _, err := h.commSvc.List(ctx, stranger, created.ID); !errors.Is(err, service.ErrForbidden) {
		t.Errorf("expected ErrForbidden for stranger List, got %v", err)
	}
}

func TestService_TaskCacheAside(t *testing.T) {
	skipIfNoDB(t)
	mc := newMemCache()
	h := openServiceHarness(t, false)
	h.cache = mc
	h.taskSvc = service.NewTaskService(h.taskRepo, h.histRepo, h.teamRepo, mc, h.tx)

	ctx := context.Background()
	owner := newUser(t, h, fmt.Sprintf("cache-%d@example.com", time.Now().UnixNano()))
	teamID := newTeam(t, h, owner, "Cache Team")

	filter := dto.TaskFilter{TeamID: teamID, Page: 1, Limit: 10}

	// До первого вызова ключа в кеше нет.
	key := fmt.Sprintf("task:list:team:%d:status:any:page:%d", teamID, filter.Page)
	if mc.has(key) {
		t.Fatalf("expected empty cache before first list")
	}

	// Первый запрос — промах, читаем из БД, складываем в кеш.
	first, err := h.taskSvc.List(ctx, owner, filter)
	if err != nil {
		t.Fatalf("List #1: %v", err)
	}
	if len(first.Items) != 0 {
		t.Errorf("first list should be empty: %+v", first.Items)
	}
	if !mc.has(key) {
		t.Errorf("expected cache miss to populate key %q", key)
	}

	// Второй запрос — должен отдать тот же объект из кеша.
	second, err := h.taskSvc.List(ctx, owner, filter)
	if err != nil {
		t.Fatalf("List #2: %v", err)
	}
	if second == first {
		t.Errorf("expected distinct pointers, got same %p", first)
	}
	if len(second.Items) != 0 {
		t.Errorf("second list should still be empty: %+v", second.Items)
	}

	if _, err := h.taskSvc.Create(ctx, owner, dto.CreateTaskRequest{
		TeamID: teamID, Title: "After invalidate", Status: entity.TaskTodo,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if mc.has(key) {
		t.Errorf("expected Invalidate to delete key %q", key)
	}

	third, err := h.taskSvc.List(ctx, owner, filter)
	if err != nil {
		t.Fatalf("List #3: %v", err)
	}
	if len(third.Items) != 1 {
		t.Errorf("expected 1 task after invalidate+create, got %d", len(third.Items))
	}
	if !mc.has(key) {
		t.Errorf("expected cache populated after post-invalidate list")
	}
}

func TestService_TaskCacheNilSafe(t *testing.T) {
	skipIfNoDB(t)
	h := openServiceHarness(t, false)

	h.taskSvc = service.NewTaskService(h.taskRepo, h.histRepo, h.teamRepo, nil, h.tx)

	ctx := context.Background()
	owner := newUser(t, h, fmt.Sprintf("nocache-%d@example.com", time.Now().UnixNano()))
	teamID := newTeam(t, h, owner, "NoCache Team")

	for i := 0; i < 3; i++ {
		if _, err := h.taskSvc.Create(ctx, owner, dto.CreateTaskRequest{
			TeamID: teamID,
			Title:  fmt.Sprintf("task %d", i),
			Status: entity.TaskTodo,
		}); err != nil {
			t.Fatalf("Create #%d: %v", i, err)
		}
	}

	resp, err := h.taskSvc.List(ctx, owner, dto.TaskFilter{TeamID: teamID, Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Items) != 3 {
		t.Errorf("expected 3 tasks without cache, got %d", len(resp.Items))
	}
	if resp.Meta.Total != 3 {
		t.Errorf("expected total=3, got %d", resp.Meta.Total)
	}
}

func skipIfNoDB(t *testing.T) {
	t.Helper()
	if os.Getenv("TEST_MYSQL_DSN") == "" && os.Getenv("MYSQL_TEST_DSN") == "" {
		t.Skip("set TEST_MYSQL_DSN or MYSQL_TEST_DSN to run integration tests")
	}
}
