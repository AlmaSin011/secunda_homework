package service

import (
	"context"
	"testing"

	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
)

func seedUser(t *testing.T, users *fakeUserRepo, email, name string) uint64 {
	t.Helper()
	id, err := users.Create(context.Background(), entity.User{
		Email: email, Name: name, PasswordHash: "x",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func TestTeamService_Create_OK(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	uid := seedUser(t, users, "alice@example.com", "Alice")

	svc := NewTeamService(teams, users, newFakeTransactor())
	got, err := svc.Create(context.Background(), uid, dto.CreateTeamRequest{Name: "Backend"})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if got.Name != "Backend" {
		t.Fatalf("name: got %q want Backend", got.Name)
	}
	if got.CreatedBy != uid {
		t.Fatalf("createdBy: got %d want %d", got.CreatedBy, uid)
	}
	// Создатель автоматически становится owner.
	m, err := teams.GetMember(context.Background(), got.ID, uid)
	if err != nil {
		t.Fatalf("owner member missing: %v", err)
	}
	if m.Role != entity.RoleOwner {
		t.Fatalf("role: got %q want owner", m.Role)
	}
}

func TestTeamService_Create_Validation(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	uid := seedUser(t, users, "a@example.com", "A")
	svc := NewTeamService(teams, users, newFakeTransactor())

	cases := []dto.CreateTeamRequest{
		{Name: ""},
		{Name: "   "},
		{Name: string(make([]byte, 101))}, // > 100
	}
	for i, req := range cases {
		if _, err := svc.Create(context.Background(), uid, req); err == nil {
			t.Fatalf("case %d: want error, got nil", i)
		} else if err != ErrValidation {
			t.Fatalf("case %d: want ErrValidation, got %v", i, err)
		}
	}
}

func TestTeamService_Create_UnknownActor(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	svc := NewTeamService(teams, users, newFakeTransactor())

	_, err := svc.Create(context.Background(), 999, dto.CreateTeamRequest{Name: "X"})
	if err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestTeamService_Invite_OK(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	owner := seedUser(t, users, "owner@example.com", "Owner")
	newbie := seedUser(t, users, "newbie@example.com", "Newbie")
	teams.seedTeam(1, "Backend", owner)
	teams.seedMember(1, owner, entity.RoleOwner)

	svc := NewTeamService(teams, users, newFakeTransactor())
	err := svc.Invite(context.Background(), owner, 1, dto.InviteRequest{
		UserID: newbie, Role: entity.RoleMember,
	})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	m, _ := teams.GetMember(context.Background(), 1, newbie)
	if m.Role != entity.RoleMember {
		t.Fatalf("role: got %q want member", m.Role)
	}
}

func TestTeamService_Invite_NotOwnerOrAdmin(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	member := seedUser(t, users, "m@example.com", "M")
	target := seedUser(t, users, "t@example.com", "T")
	teams.seedTeam(1, "Team", owner)
	teams.seedMember(1, owner, entity.RoleOwner)
	teams.seedMember(1, member, entity.RoleMember)

	svc := NewTeamService(teams, users, newFakeTransactor())
	err := svc.Invite(context.Background(), member, 1, dto.InviteRequest{
		UserID: target, Role: entity.RoleMember,
	})
	if err != ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestTeamService_Invite_TeamNotFound(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	target := seedUser(t, users, "t@example.com", "T")
	svc := NewTeamService(teams, users, newFakeTransactor())

	err := svc.Invite(context.Background(), owner, 999, dto.InviteRequest{
		UserID: target, Role: entity.RoleMember,
	})
	if err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestTeamService_Invite_AlreadyExists(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	existing := seedUser(t, users, "e@example.com", "E")
	teams.seedTeam(1, "Team", owner)
	teams.seedMember(1, owner, entity.RoleOwner)
	teams.seedMember(1, existing, entity.RoleMember)

	svc := NewTeamService(teams, users, newFakeTransactor())
	err := svc.Invite(context.Background(), owner, 1, dto.InviteRequest{
		UserID: existing, Role: entity.RoleAdmin,
	})
	if err != ErrAlreadyExists {
		t.Fatalf("want ErrAlreadyExists, got %v", err)
	}
}

func TestTeamService_Invite_BadRole(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	target := seedUser(t, users, "t@example.com", "T")
	teams.seedTeam(1, "Team", owner)
	teams.seedMember(1, owner, entity.RoleOwner)

	svc := NewTeamService(teams, users, newFakeTransactor())
	err := svc.Invite(context.Background(), owner, 1, dto.InviteRequest{
		UserID: target, Role: entity.Role("hacker"),
	})
	if err != ErrValidation {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestTeamService_Invite_UnknownUser(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	teams.seedTeam(1, "Team", owner)
	teams.seedMember(1, owner, entity.RoleOwner)

	svc := NewTeamService(teams, users, newFakeTransactor())
	err := svc.Invite(context.Background(), owner, 1, dto.InviteRequest{
		UserID: 999, Role: entity.RoleMember,
	})
	if err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestTeamService_List_OK(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	uid := seedUser(t, users, "u@example.com", "U")
	teams.seedTeam(1, "T1", uid)
	teams.seedTeam(2, "T2", uid)
	teams.seedMember(1, uid, entity.RoleOwner)
	teams.seedMember(2, uid, entity.RoleMember)

	svc := NewTeamService(teams, users, newFakeTransactor())
	out, err := svc.List(context.Background(), uid)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len: got %d want 2", len(out))
	}
}

func TestTeamService_ListMembers_ForbiddenIfNotMember(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	stranger := seedUser(t, users, "s@example.com", "S")
	teams.seedTeam(1, "T", owner)
	teams.seedMember(1, owner, entity.RoleOwner)

	svc := NewTeamService(teams, users, newFakeTransactor())
	_, err := svc.ListMembers(context.Background(), stranger, 1)
	if err != ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestTeamService_ListMembers_OK(t *testing.T) {
	users := newFakeUserRepo()
	teams := newFakeTeamRepo()
	owner := seedUser(t, users, "o@example.com", "O")
	teams.seedTeam(1, "T", owner)
	teams.seedMember(1, owner, entity.RoleOwner)
	teams.seedMember(1, 42, entity.RoleMember)

	svc := NewTeamService(teams, users, newFakeTransactor())
	out, err := svc.ListMembers(context.Background(), owner, 1)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len: got %d want 2", len(out))
	}
}
