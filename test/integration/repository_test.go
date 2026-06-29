//go:build integration

package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/repository"
	"github.com/jmoiron/sqlx"

	_ "github.com/go-sql-driver/mysql"
)

const fixedTimestamp = "2026-06-15 12:00:00"

func mustOpen(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_TEST_DSN")
	}
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN/MYSQL_TEST_DSN not set; skipping integration test")
	}
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		t.Skipf("MySQL not reachable: %v", err)
	}
	db.SetMaxOpenConns(5)
	return db
}

func cleanup(t *testing.T, db *sqlx.DB) {
	t.Helper()
	_, _ = db.Exec("DELETE FROM task_comments")
	_, _ = db.Exec("DELETE FROM task_history")
	_, _ = db.Exec("DELETE FROM tasks")
	_, _ = db.Exec("DELETE FROM team_members")
	_, _ = db.Exec("DELETE FROM teams")
	_, _ = db.Exec("DELETE FROM users")
}

func ts() time.Time {
	v, _ := time.Parse("2006-01-02 15:04:05", fixedTimestamp)
	return v
}

func TestUserRepository(t *testing.T) {
	db := mustOpen(t)
	defer db.Close()
	defer cleanup(t, db)

	ctx := context.Background()
	users := repository.NewUserRepository(db)

	u := entity.User{
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Name:         "Alice",
		CreatedAt:    ts(),
		UpdatedAt:    ts(),
	}
	id, err := users.Create(ctx, u)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == 0 {
		t.Fatalf("Create returned 0 id")
	}

	got, err := users.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("email mismatch: %q", got.Email)
	}

	gotByEmail, err := users.FindByEmail(ctx, "ALICE@example.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if gotByEmail == nil || gotByEmail.ID != id {
		t.Errorf("FindByEmail mismatch")
	}

	if _, err := users.Create(ctx, u); !errors.Is(err, repository.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}

	if u2, err := users.FindByEmail(ctx, "nobody@example.com"); err != nil || u2 != nil {
		t.Errorf("expected (nil, nil), got (%v, %v)", u2, err)
	}

	if _, err := users.FindByID(ctx, 999_999); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTeamRepository(t *testing.T) {
	db := mustOpen(t)
	defer db.Close()
	defer cleanup(t, db)
	ctx := context.Background()

	users := repository.NewUserRepository(db)
	teams := repository.NewTeamRepository(db)

	ownerID, _ := users.Create(ctx, entity.User{
		Email:        "owner@example.com",
		PasswordHash: "h",
		Name:         "Owner",
		CreatedAt:    ts(),
		UpdatedAt:    ts(),
	})
	memberID, _ := users.Create(ctx, entity.User{
		Email:        "member@example.com",
		PasswordHash: "h",
		Name:         "Member",
		CreatedAt:    ts(),
		UpdatedAt:    ts(),
	})

	teamID, err := teams.Create(ctx, entity.Team{
		Name:      "Team A",
		CreatedBy: ownerID,
		CreatedAt: ts(),
		UpdatedAt: ts(),
	})
	if err != nil {
		t.Fatalf("Create team: %v", err)
	}

	if err := teams.AddMember(ctx, entity.TeamMember{
		UserID: ownerID, TeamID: teamID, Role: entity.RoleOwner, JoinedAt: ts(),
	}); err != nil {
		t.Fatalf("AddMember owner: %v", err)
	}
	if err := teams.AddMember(ctx, entity.TeamMember{
		UserID: memberID, TeamID: teamID, Role: entity.RoleMember, JoinedAt: ts(),
	}); err != nil {
		t.Fatalf("AddMember member: %v", err)
	}

	if err := teams.AddMember(ctx, entity.TeamMember{
		UserID: ownerID, TeamID: teamID, Role: entity.RoleOwner, JoinedAt: ts(),
	}); !errors.Is(err, repository.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists on duplicate AddMember, got %v", err)
	}

	if ok, err := teams.UserBelongsToTeam(ctx, teamID, memberID); err != nil || !ok {
		t.Errorf("expected member to belong: ok=%v err=%v", ok, err)
	}
	if ok, _ := teams.UserBelongsToTeam(ctx, teamID, 999_999); ok {
		t.Errorf("expected non-member to not belong")
	}

	members, err := teams.ListMembers(ctx, teamID)
	if err != nil || len(members) != 2 {
		t.Fatalf("ListMembers: len=%d err=%v", len(members), err)
	}

	if err := teams.UpdateMemberRole(ctx, teamID, memberID, entity.RoleAdmin); err != nil {
		t.Fatalf("UpdateMemberRole: %v", err)
	}
	got, err := teams.GetMember(ctx, teamID, memberID)
	if err != nil || got.Role != entity.RoleAdmin {
		t.Errorf("expected role=admin, got %v err=%v", got.Role, err)
	}

	list, err := teams.ListByUser(ctx, memberID)
	if err != nil || len(list) != 1 || list[0].Role != entity.RoleAdmin {
		t.Errorf("ListByUser: %v", list)
	}

	if err := teams.RemoveMember(ctx, teamID, memberID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if err := teams.RemoveMember(ctx, teamID, memberID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound on second RemoveMember, got %v", err)
	}
}

func TestTaskRepositorySoftDelete(t *testing.T) {
	db := mustOpen(t)
	defer db.Close()
	defer cleanup(t, db)
	ctx := context.Background()

	users := repository.NewUserRepository(db)
	teams := repository.NewTeamRepository(db)
	tasks := repository.NewTaskRepository(db)

	uid, _ := users.Create(ctx, entity.User{Email: "u@x.com", PasswordHash: "h", Name: "U", CreatedAt: ts(), UpdatedAt: ts()})
	tid, _ := teams.Create(ctx, entity.Team{Name: "T", CreatedBy: uid, CreatedAt: ts(), UpdatedAt: ts()})
	_ = teams.AddMember(ctx, entity.TeamMember{UserID: uid, TeamID: tid, Role: entity.RoleOwner, JoinedAt: ts()})

	desc := "desc"
	id, err := tasks.Create(ctx, entity.Task{
		TeamID:      tid,
		Title:       "task",
		Description: &desc,
		Status:      entity.TaskTodo,
		CreatedBy:   uid,
		AssigneeID:  ptrUint64(uid),
		CreatedAt:   ts(),
		UpdatedAt:   ts(),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := tasks.FindByID(ctx, id)
	if err != nil || got.Title != "task" {
		t.Fatalf("FindByID: %v", err)
	}

	list, err := tasks.List(ctx, dto_TeamFilter(tid))
	if err != nil || len(list) != 1 {
		t.Errorf("List: len=%d err=%v", len(list), err)
	}

	title := "patched"
	updated, err := tasks.Update(ctx, id, repository.TaskPatch{Title: &title, ClearAssignee: true})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "patched" {
		t.Errorf("title not updated: %q", updated.Title)
	}
	if updated.AssigneeID != nil {
		t.Errorf("expected nil assignee, got %v", updated.AssigneeID)
	}

	if err := tasks.SoftDelete(ctx, id); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	listAfter, _ := tasks.List(ctx, dto_TeamFilter(tid))
	if len(listAfter) != 0 {
		t.Errorf("expected empty list after soft delete, got %d", len(listAfter))
	}

	if err := tasks.SoftDelete(ctx, id); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound on second soft delete, got %v", err)
	}
}

func TestStatsQueries(t *testing.T) {
	db := mustOpen(t)
	defer db.Close()
	defer cleanup(t, db)
	ctx := context.Background()

	users := repository.NewUserRepository(db)
	teams := repository.NewTeamRepository(db)
	tasks := repository.NewTaskRepository(db)
	history := repository.NewHistoryRepository(db)
	stats := repository.NewStatsRepository(db)

	ownerID, _ := users.Create(ctx, entity.User{Email: "owner@fddf.com", PasswordHash: "h", Name: "Owner", CreatedAt: ts(), UpdatedAt: ts()})
	creatorID, _ := users.Create(ctx, entity.User{Email: "creator@gfdfr.com", PasswordHash: "h", Name: "Creator", CreatedAt: ts(), UpdatedAt: ts()})

	teamID, _ := teams.Create(ctx, entity.Team{Name: "Stats", CreatedBy: ownerID, CreatedAt: ts(), UpdatedAt: ts()})
	if err := teams.AddMember(ctx, entity.TeamMember{UserID: ownerID, TeamID: teamID, Role: entity.RoleOwner, JoinedAt: ts()}); err != nil {
		t.Fatalf("AddMember owner: %v", err)
	}

	now := time.Now()
	taskID, err := tasks.Create(ctx, entity.Task{
		TeamID:     teamID,
		Title:      "t1",
		Status:     entity.TaskTodo,
		CreatedBy:  creatorID,
		AssigneeID: ptrUint64(creatorID),
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatalf("Create task: %v", err)
	}

	recent := now.Add(-1 * time.Hour)
	oldVal := "todo"
	newVal := "done"
	if err := history.Insert(ctx, entity.TaskHistory{
		TaskID:    taskID,
		ChangedBy: ownerID,
		Field:     "status",
		OldValue:  &oldVal,
		NewValue:  &newVal,
		ChangedAt: recent,
	}); err != nil {
		t.Fatalf("Insert history: %v", err)
	}

	statsTeam, err := stats.TeamStatsLastWeek(ctx)
	if err != nil {
		t.Fatalf("TeamStatsLastWeek: %v", err)
	}

	var ours *dto.TeamStatsResponse
	for i := range statsTeam {
		if statsTeam[i].TeamID == teamID {
			ours = &statsTeam[i]
			break
		}
	}
	if ours == nil {
		t.Fatalf("team %d not in stats result: %+v", teamID, statsTeam)
	}
	if ours.MemberCount != 1 {
		t.Errorf("expected MemberCount=1, got %d", ours.MemberCount)
	}
	if ours.DoneLast7Days != 1 {
		t.Errorf("expected DoneLast7Days=1, got %d", ours.DoneLast7Days)
	}
	if ours.TeamName != "Stats" {
		t.Errorf("expected TeamName=Stats, got %q", ours.TeamName)
	}

	orphans, err := stats.OrphanTasks(ctx)
	if err != nil {
		t.Fatalf("OrphanTasks: %v", err)
	}
	var orphan *dto.OrphanTaskResponse
	for i := range orphans {
		if orphans[i].TaskID == taskID {
			orphan = &orphans[i]
			break
		}
	}
	if orphan == nil {
		t.Fatalf("expected taskID %d in orphans, got %+v", taskID, orphans)
	}
	if orphan.AssigneeID == nil || *orphan.AssigneeID != creatorID {
		t.Errorf("expected AssigneeID=%d, got %v", creatorID, orphan.AssigneeID)
	}
	if orphan.AssigneeEmail == nil || *orphan.AssigneeEmail != "creator@gfdfr.com" {
		t.Errorf("expected AssigneeEmail=creator@gfdfr.com, got %v", orphan.AssigneeEmail)
	}

	top, err := stats.TopCreatorsByTeam(ctx, 7, 5)
	if err != nil {
		t.Fatalf("TopCreatorsByTeam: %v", err)
	}
	var oursTop *dto.TopCreatorEntry
	for i := range top {
		if top[i].TeamID == teamID && top[i].UserID == creatorID {
			oursTop = &top[i]
			break
		}
	}
	if oursTop == nil {
		t.Fatalf("expected creator %d in team %d TopCreatorsByTeam, got %+v", creatorID, teamID, top)
	}
	if oursTop.TaskCount != 1 {
		t.Errorf("expected TaskCount=1, got %d", oursTop.TaskCount)
	}
	if oursTop.Rank != 1 {
		t.Errorf("expected Rank=1, got %d", oursTop.Rank)
	}
}
