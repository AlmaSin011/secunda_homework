package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/repository"
)

func ptr(s string) *string { return &s }

func now() time.Time { return time.Now() }

func TestCommentService_Create_OK(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	comments := newFakeCommentRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	svc := NewCommentService(comments, tasks, teams, newFakeTransactor())
	id, err := svc.Create(context.Background(), uid, 1, dto.CreateCommentRequest{Body: "hi"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id == 0 {
		t.Fatalf("id == 0")
	}
}

func TestCommentService_Create_Validation(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	comments := newFakeCommentRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})
	svc := NewCommentService(comments, tasks, teams, newFakeTransactor())

	cases := []dto.CreateCommentRequest{
		{Body: ""},
		{Body: "   "},
		{Body: strings.Repeat("a", 4001)},
	}
	for i, req := range cases {
		if _, err := svc.Create(context.Background(), uid, 1, req); err != ErrValidation {
			t.Fatalf("case %d: want ErrValidation, got %v", i, err)
		}
	}
}

func TestCommentService_Create_NotMember(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	comments := newFakeCommentRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	stranger := seedUser(t, users, "s@example.com", "S")
	teams.seedTeam(1, "T", owner)
	teams.seedMember(1, owner, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: owner})

	svc := NewCommentService(comments, tasks, teams, newFakeTransactor())
	_, err := svc.Create(context.Background(), stranger, 1, dto.CreateCommentRequest{Body: "hi"})
	if err != ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestCommentService_Create_TaskNotFound(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	comments := newFakeCommentRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	svc := NewCommentService(comments, tasks, teams, newFakeTransactor())
	_, err := svc.Create(context.Background(), uid, 999, dto.CreateCommentRequest{Body: "hi"})
	if err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCommentService_List_OK(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	comments := newFakeCommentRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})
	_, _ = comments.Create(context.Background(), entity.TaskComment{TaskID: 1, UserID: uid, Body: "a"})
	_, _ = comments.Create(context.Background(), entity.TaskComment{TaskID: 1, UserID: uid, Body: "b"})

	svc := NewCommentService(comments, tasks, teams, newFakeTransactor())
	out, err := svc.List(context.Background(), uid, 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len: got %d want 2", len(out))
	}
}

func TestCommentService_List_Forbidden(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	comments := newFakeCommentRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	stranger := seedUser(t, users, "s@example.com", "S")
	teams.seedTeam(1, "T", owner)
	teams.seedMember(1, owner, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: owner})
	svc := NewCommentService(comments, tasks, teams, newFakeTransactor())

	_, err := svc.List(context.Background(), stranger, 1)
	if err != ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestStatsService_LastWeek_OK(t *testing.T) {
	stats := newFakeStatsRepo()
	stats.teamStats = []dto.TeamStatsResponse{{TeamID: 1, TeamName: "T", MemberCount: 3, DoneLast7Days: 5}}

	svc := NewStatsService(stats)
	out, err := svc.LastWeek(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) != 1 || out[0].DoneLast7Days != 5 {
		t.Fatalf("out: %+v", out)
	}
	if stats.teamStatsCalls.Load() != 1 {
		t.Fatalf("calls: got %d", stats.teamStatsCalls.Load())
	}
}

func TestStatsService_TopCreators_Normalization(t *testing.T) {
	stats := newFakeStatsRepo()
	svc := NewStatsService(stats)

	_, _ = svc.TopCreators(context.Background(), -1, 0) // sinceDays<=0, limit<=0
	if stats.lastSinceDays.Load() != 30 || stats.lastLimit.Load() != 10 {
		t.Fatalf("norm: since=%d limit=%d",
			stats.lastSinceDays.Load(), stats.lastLimit.Load())
	}

	_, _ = svc.TopCreators(context.Background(), 1, 9999) // limit > 100 → 10
	if stats.lastLimit.Load() != 10 {
		t.Fatalf("limit clamp: got %d", stats.lastLimit.Load())
	}

	_, _ = svc.TopCreators(context.Background(), 7, 5)
	if stats.lastSinceDays.Load() != 7 || stats.lastLimit.Load() != 5 {
		t.Fatalf("passthrough: since=%d limit=%d",
			stats.lastSinceDays.Load(), stats.lastLimit.Load())
	}
}

func TestStatsService_Orphans_OK(t *testing.T) {
	stats := newFakeStatsRepo()
	stats.orphanTasks = []dto.OrphanTaskResponse{{TaskID: 1, TeamID: 1, Title: "t"}}
	svc := NewStatsService(stats)

	out, err := svc.Orphans(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) != 1 || out[0].TaskID != 1 {
		t.Fatalf("out: %+v", out)
	}
	if stats.orphanCalls.Load() != 1 {
		t.Fatalf("calls: got %d", stats.orphanCalls.Load())
	}
}

func TestCommentService_List_Empty(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	comments := newFakeCommentRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	svc := NewCommentService(comments, tasks, teams, newFakeTransactor())
	out, err := svc.List(context.Background(), uid, 1)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("len: got %d want 0", len(out))
	}
}

func TestCommentService_List_TaskNotFound(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	comments := newFakeCommentRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	svc := NewCommentService(comments, tasks, teams, newFakeTransactor())

	_, err := svc.List(context.Background(), uid, 999)
	if err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// прокси с ошибкой.
type errCommentRepo struct{ err error }

func (r *errCommentRepo) Create(_ context.Context, _ entity.TaskComment) (uint64, error) {
	return 0, r.err
}
func (r *errCommentRepo) InsertTx(_ context.Context, _ TxExec, _ entity.TaskComment) (uint64, error) {
	return 0, r.err
}
func (r *errCommentRepo) ListByTask(_ context.Context, _ uint64) ([]entity.TaskComment, error) {
	return nil, r.err
}

func TestCommentService_List_RepoError(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	svc := NewCommentService(&errCommentRepo{err: errors.New("db down")}, tasks, teams, newFakeTransactor())
	_, err := svc.List(context.Background(), uid, 1)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestCommentService_Create_RepoError(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	tasks := newFakeTaskRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	tasks.seedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	svc := NewCommentService(&errCommentRepo{err: errors.New("db down")}, tasks, teams, newFakeTransactor())
	_, err := svc.Create(context.Background(), uid, 1, dto.CreateCommentRequest{Body: "hi"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestTeamService_Create_AddMemberFails(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	// Делаем AddMember фейковой ошибкой через обёртку.
	bad := &errTeamRepo{teams: teams, err: errors.New("db down")}
	svc := NewTeamService(bad, users, newFakeTransactor())
	if _, err := svc.Create(context.Background(), uid, dto.CreateTeamRequest{Name: "X"}); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestTeamService_List_Empty(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	svc := NewTeamService(teams, users, newFakeTransactor())
	out, err := svc.List(context.Background(), uid)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("len: got %d want 0", len(out))
	}
}

func TestTeamService_Invite_FindByIDFails(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	target := seedUser(t, users, "t@example.com", "T")
	teams.seedTeam(1, "Team", owner)
	teams.seedMember(1, owner, entity.RoleOwner)

	bad := &errUserLookup{err: errors.New("db down")}
	svc := NewTeamService(teams, bad, newFakeTransactor())
	err := svc.Invite(context.Background(), owner, 1, dto.InviteRequest{
		UserID: target, Role: entity.RoleMember,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestTeamService_Invite_GetMemberFails(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	target := seedUser(t, users, "t@example.com", "T")
	teams.seedTeam(1, "Team", owner)
	teams.seedMember(1, owner, entity.RoleOwner)

	bad := &errTeamRepo{teams: teams, err: errors.New("db down")}
	svc := NewTeamService(bad, users, newFakeTransactor())
	err := svc.Invite(context.Background(), owner, 1, dto.InviteRequest{
		UserID: target, Role: entity.RoleMember,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestTeamService_List_RepoError(t *testing.T) {
	users := newFakeUserRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	bad := &errTeamRepo{err: errors.New("db down")}
	svc := NewTeamService(bad, users, newFakeTransactor())
	if _, err := svc.List(context.Background(), uid); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestTeamService_ListMembers_RepoError(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	teams.seedTeam(1, "T", owner)
	teams.seedMember(1, owner, entity.RoleOwner)

	bad := &errTeamRepo{teams: teams, err: errors.New("db down")}
	svc := NewTeamService(bad, users, newFakeTransactor())
	if _, err := svc.ListMembers(context.Background(), owner, 1); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

// errTeamRepo — прокси поверх fakeTeamRepo: возвращает ошибку на всех операциях.
type errTeamRepo struct {
	teams *fakeTeamRepo
	err   error
}

func (r *errTeamRepo) Create(_ context.Context, _ entity.Team) (uint64, error) { return 0, r.err }
func (r *errTeamRepo) FindByID(_ context.Context, _ uint64) (*entity.Team, error) {
	return nil, r.err
}
func (r *errTeamRepo) ListByUser(_ context.Context, _ uint64) ([]repository.TeamWithRole, error) {
	return nil, r.err
}
func (r *errTeamRepo) AddMember(_ context.Context, _ entity.TeamMember) error { return r.err }
func (r *errTeamRepo) GetMember(_ context.Context, _, _ uint64) (*entity.TeamMember, error) {
	return nil, r.err
}
func (r *errTeamRepo) ListMembers(_ context.Context, _ uint64) ([]entity.TeamMember, error) {
	return nil, r.err
}

type errUserLookup struct{ err error }

func (r *errUserLookup) FindByID(_ context.Context, _ uint64) (*entity.User, error) {
	return nil, r.err
}
