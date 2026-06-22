package entity

import (
	"errors"
	"testing"
)

func TestParseRole(t *testing.T) {
	tests := []struct {
		in      string
		want    Role
		wantErr bool
	}{
		{"owner", RoleOwner, false},
		{"admin", RoleAdmin, false},
		{"member", RoleMember, false},
		// Регистр — не важен.
		{"OWNER", RoleOwner, false},
		{"Admin", RoleAdmin, false},
		{"MeMbEr", RoleMember, false},
		// Мусор.
		{"", "", true},
		{"superuser", "", true},
		{"owner admin", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseRole(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRole(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseRole(%q)=%q want %q", tt.in, got, tt.want)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidRole) {
				t.Errorf("err should wrap ErrInvalidRole, got %v", err)
			}
		})
	}
}

func TestRoleCanInvite(t *testing.T) {
	if !RoleOwner.CanInvite() {
		t.Error("owner must be able to invite")
	}
	if !RoleAdmin.CanInvite() {
		t.Error("admin must be able to invite")
	}
	if RoleMember.CanInvite() {
		t.Error("member must NOT be able to invite")
	}
	if Role("garbage").CanInvite() {
		t.Error("invalid role must not pass CanInvite")
	}
}

func TestRoleIsValid(t *testing.T) {
	for _, r := range ValidRoles {
		if !r.IsValid() {
			t.Errorf("%q should be valid", r)
		}
	}
	if Role("xxx").IsValid() {
		t.Error("'xxx' must be invalid")
	}
}

func TestParseTaskStatus(t *testing.T) {
	tests := []struct {
		in      string
		want    TaskStatus
		wantErr bool
	}{
		{"todo", TaskTodo, false},
		{"in_progress", TaskInProgress, false},
		{"done", TaskDone, false},
		// Регистр.
		{"TODO", TaskTodo, false},
		{"In_Progress", TaskInProgress, false},
		// Мусор.
		{"", "", true},
		{"completed", "", true},
		{"todo ", "", true}, // пробел не триммится — это валидация status, не title
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseTaskStatus(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseTaskStatus(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidTaskStatus) {
				t.Errorf("err should wrap ErrInvalidTaskStatus, got %v", err)
			}
		})
	}
}

func TestTaskStatusIsValid(t *testing.T) {
	for _, s := range ValidStatuses {
		if !s.IsValid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if TaskStatus("archived").IsValid() {
		t.Error("'archived' must be invalid")
	}
}
