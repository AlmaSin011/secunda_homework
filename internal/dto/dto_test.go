package dto

import (
	"errors"
	"strings"
	"testing"

	"github.com/example/go-project/internal/entity"
)

func TestRegisterRequest_Validate(t *testing.T) {
	cases := []struct {
		name string
		req  RegisterRequest
		ok   bool
	}{
		{"valid", RegisterRequest{Email: "a@b.c", Password: "12345678", Name: "Alice"}, true},
		{"trims name", RegisterRequest{Email: "a@b.c", Password: "12345678", Name: "  Bob  "}, true},
		{"no email", RegisterRequest{Password: "12345678", Name: "X"}, false},
		{"bad email", RegisterRequest{Email: "not-an-email", Password: "12345678", Name: "X"}, false},
		{"no password", RegisterRequest{Email: "a@b.c", Name: "X"}, false},
		{"short password", RegisterRequest{Email: "a@b.c", Password: "short", Name: "X"}, false},
		{"long password", RegisterRequest{Email: "a@b.c", Password: strings.Repeat("a", 73), Name: "X"}, false},
		{"no name", RegisterRequest{Email: "a@b.c", Password: "12345678"}, false},
		{"long name", RegisterRequest{Email: "a@b.c", Password: "12345678", Name: strings.Repeat("x", MaxNameLen+1)}, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err == nil) != tt.ok {
				t.Errorf("Validate err=%v, ok=%v", err, tt.ok)
			}
		})
	}
}

func TestLoginRequest_Validate(t *testing.T) {
	if err := (LoginRequest{Email: "a@b.c", Password: "x"}).Validate(); err != nil {
		t.Errorf("valid login: %v", err)
	}
	if err := (LoginRequest{Email: "", Password: "x"}).Validate(); err == nil {
		t.Error("empty email should fail")
	}
	if err := (LoginRequest{Email: "bad", Password: "x"}).Validate(); err == nil {
		t.Error("invalid email should fail")
	}
}

func TestCreateTeamRequest_Validate(t *testing.T) {
	if err := (CreateTeamRequest{Name: "Engineering"}).Validate(); err != nil {
		t.Errorf("valid team: %v", err)
	}
	if err := (CreateTeamRequest{Name: "   "}).Validate(); err == nil {
		t.Error("blank name should fail after trim")
	}
	if err := (CreateTeamRequest{Name: strings.Repeat("x", MaxNameLen+1)}).Validate(); err == nil {
		t.Error("overlong name should fail")
	}
}

func TestInviteRequest_Validate(t *testing.T) {
	good := InviteRequest{UserID: 7, Role: entity.RoleMember}
	if err := good.Validate(); err != nil {
		t.Errorf("valid invite: %v", err)
	}
	zero := InviteRequest{UserID: 0, Role: entity.RoleAdmin}
	if err := zero.Validate(); !errors.Is(err, ErrZeroUserID) {
		t.Errorf("zero user_id: got %v, want ErrZeroUserID", err)
	}
	bad := InviteRequest{UserID: 7, Role: "god"}
	if err := bad.Validate(); !errors.Is(err, ErrInvalidRole) {
		t.Errorf("bad role: got %v, want ErrInvalidRole", err)
	}
	// Парсинг в нижний регистр.
	upper := InviteRequest{UserID: 7, Role: "Owner"}
	if err := upper.Validate(); err != nil {
		t.Fatalf("uppercase role: %v", err)
	}
	if upper.Role != entity.RoleOwner {
		t.Errorf("role not lowercased: %q", upper.Role)
	}
}

func TestCreateTaskRequest_Validate(t *testing.T) {
	good := CreateTaskRequest{TeamID: 1, Title: "Do stuff"}
	if err := good.Validate(); err != nil {
		t.Errorf("valid create: %v", err)
	}
	zero := CreateTaskRequest{TeamID: 0, Title: "x"}
	if err := zero.Validate(); !errors.Is(err, ErrZeroTeamID) {
		t.Errorf("zero team: got %v", err)
	}
	blank := CreateTaskRequest{TeamID: 1, Title: "  "}
	if err := blank.Validate(); !errors.Is(err, ErrEmptyTaskTitle) {
		t.Errorf("blank title: got %v", err)
	}
	long := CreateTaskRequest{TeamID: 1, Title: strings.Repeat("x", MaxTitleLen+1)}
	if err := long.Validate(); !errors.Is(err, ErrLongTaskTitle) {
		t.Errorf("long title: got %v", err)
	}
	// Статус опционален — дефолт todo.
	def := CreateTaskRequest{TeamID: 1, Title: "X"}
	if err := def.Validate(); err != nil {
		t.Fatalf("default status: %v", err)
	}
	if def.Status != entity.TaskTodo {
		t.Errorf("default status: got %q, want todo", def.Status)
	}
	// Невалидный статус.
	bad := CreateTaskRequest{TeamID: 1, Title: "X", Status: "archived"}
	if err := bad.Validate(); !errors.Is(err, ErrInvalidStatus) {
		t.Errorf("bad status: got %v", err)
	}
}

func TestUpdateTaskRequest_Validate(t *testing.T) {
	// Пустое обновление — допустимо, но HasAnyUpdate==false.
	req := UpdateTaskRequest{}
	if err := req.Validate(); err != nil {
		t.Errorf("empty update: %v", err)
	}
	if req.HasAnyUpdate() {
		t.Error("empty request should report no updates")
	}
	// Только title.
	title := "new title"
	if err := (UpdateTaskRequest{Title: &title}).Validate(); err != nil {
		t.Errorf("title-only update: %v", err)
	}
	// Пустой title.
	empty := ""
	if err := (UpdateTaskRequest{Title: &empty}).Validate(); !errors.Is(err, ErrEmptyTaskTitle) {
		t.Errorf("blank title: got %v", err)
	}
	// Невалидный статус.
	bad := entity.TaskStatus("archived")
	if err := (UpdateTaskRequest{Status: &bad}).Validate(); !errors.Is(err, ErrInvalidStatus) {
		t.Errorf("bad status: got %v", err)
	}
}

func TestCreateCommentRequest_Validate(t *testing.T) {
	if err := (CreateCommentRequest{Body: "looks good"}).Validate(); err != nil {
		t.Errorf("valid comment: %v", err)
	}
	if err := (CreateCommentRequest{Body: "   "}).Validate(); !errors.Is(err, ErrEmptyCommentBody) {
		t.Errorf("blank body: got %v", err)
	}
	if err := (CreateCommentRequest{Body: strings.Repeat("a", MaxBodyLen+1)}).Validate(); !errors.Is(err, ErrLongCommentBody) {
		t.Errorf("long body: got %v", err)
	}
}

func TestBindTaskFilter(t *testing.T) {
	q := map[string]string{
		"team_id":     "42",
		"status":      "DONE",
		"assignee_id": "7",
		"page":        "3",
		"limit":       "200", // превысит MaxPageLimit — должно зажаться
	}
	get := func(k string) string { return q[k] }
	f := BindTaskFilter(get)
	if f.TeamID != 42 {
		t.Errorf("TeamID=%d", f.TeamID)
	}
	if f.Status != entity.TaskDone {
		t.Errorf("Status=%q", f.Status)
	}
	if f.AssigneeID == nil || *f.AssigneeID != 7 {
		t.Errorf("AssigneeID=%v", f.AssigneeID)
	}
	if f.Page != 3 {
		t.Errorf("Page=%d", f.Page)
	}
	if f.Limit != MaxPageLimit {
		t.Errorf("Limit=%d, want %d", f.Limit, MaxPageLimit)
	}
	if f.Offset() != 2*f.Limit {
		t.Errorf("Offset=%d, want %d", f.Offset(), 2*f.Limit)
	}
}

func TestBindTaskFilter_DefaultsAndBadInputs(t *testing.T) {
	get := func(string) string { return "" }
	f := BindTaskFilter(get)
	if f.Page != 1 || f.Limit != DefaultPageLimit {
		t.Errorf("defaults: page=%d limit=%d", f.Page, f.Limit)
	}
	if f.Status != "" {
		t.Errorf("Status=%q, want empty", f.Status)
	}

	// Невалидный status — тихо игнорируется.
	q := map[string]string{"status": "archived", "team_id": "abc", "page": "-1", "limit": "0"}
	f = BindTaskFilter(func(k string) string { return q[k] })
	if f.Status != "" {
		t.Errorf("bad status: got %q", f.Status)
	}
	if f.TeamID != 0 {
		t.Errorf("bad team_id: got %d", f.TeamID)
	}
	if f.Page != 1 {
		t.Errorf("bad page: got %d", f.Page)
	}
	// limit=0 → нормализуется в MinPageLimit.
	if f.Limit != MinPageLimit {
		t.Errorf("zero limit: got %d, want %d", f.Limit, MinPageLimit)
	}
}

func TestNewDataAndNewError(t *testing.T) {
	d := NewData("ok")
	if d.Data != "ok" || d.Error != nil {
		t.Errorf("NewData: %+v", d)
	}
	e := NewError(CodeNotFound, "task 1")
	if e.Data != nil || e.Error == nil || e.Error.Code != CodeNotFound {
		t.Errorf("NewError: %+v", e)
	}
}
