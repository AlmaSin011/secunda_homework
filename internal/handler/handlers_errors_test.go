// handlers_errors_test.go — error-path coverage для handler-ов.
package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/example/go-project/internal/auth"
	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/handler"
	"github.com/example/go-project/internal/middleware"
	"github.com/example/go-project/internal/service"
	"github.com/example/go-project/internal/testkit"
)

func errorFromEnvelope(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var env struct {
		Data  map[string]any `json:"data"`
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v body=%s", err, string(body))
	}
	if env.Error == nil {
		t.Fatalf("expected error envelope, got data=%v body=%s", env.Data, string(body))
	}
	return env.Error
}

// Прогоняется через TaskHandler.Get, у которого ветка ErrNotFound встречается,
// когда запрашиваемой задачи нет.
func TestWriteServiceError_NotFound(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/tasks/123", tok)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
	env := errorFromEnvelope(t, w.Body.Bytes())
	if env["code"] != "not_found" {
		t.Fatalf("code: %v", env["code"])
	}
}

func TestWriteServiceError_AlreadyExists(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	target := e.users.Seed("t@example.com", "T")
	tok := tokenForID(t, owner)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)
	// Уже добавлен → повторный invite должен дать 409.
	e.teams.SeedMember(1, target, entity.RoleMember)

	w := doJSON(e.router, http.MethodPost, "/api/v1/teams/1/invite", tok, dto.InviteRequest{
		UserID: target, Role: entity.RoleMember,
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d body=%s", w.Code, w.Body.String())
	}
	env := errorFromEnvelope(t, w.Body.Bytes())
	if env["code"] != "conflict" {
		t.Fatalf("code: %v", env["code"])
	}
}

func TestWriteServiceError_NotTeamMember(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	stranger := e.users.Seed("s@example.com", "S")
	tok := tokenForID(t, stranger)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)

	w := doJSON(e.router, http.MethodPost, "/api/v1/teams/1/invite", tok, dto.InviteRequest{
		UserID: 99, Role: entity.RoleMember,
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
	env := errorFromEnvelope(t, w.Body.Bytes())
	if env["code"] != "forbidden" {
		t.Fatalf("code: %v", env["code"])
	}
}

func TestTaskHandler_Update_NoFieldsEmpty(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)
	e.tasks.SeedTask(1, entity.Task{TeamID: teamID, Title: "old", Status: entity.TaskTodo, CreatedBy: uid})

	empty := "   "
	w := doJSON(e.router, http.MethodPut, "/api/v1/tasks/1", tok, dto.UpdateTaskRequest{Title: &empty})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_Update_NotTeamMember — actor не состоит в команде задачи → 403.
func TestTaskHandler_Update_NotTeamMember(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	stranger := e.users.Seed("s@example.com", "S")
	tok := tokenForID(t, stranger)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)
	e.tasks.SeedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: owner})

	newTitle := "new"
	w := doJSON(e.router, http.MethodPut, "/api/v1/tasks/1", tok, dto.UpdateTaskRequest{Title: &newTitle})
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_Update_NotFound — задача не существует → 404.
func TestTaskHandler_Update_NotFound(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	newTitle := "new"
	w := doJSON(e.router, http.MethodPut, "/api/v1/tasks/999", tok, dto.UpdateTaskRequest{Title: &newTitle})
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_Delete_NotFound — soft-delete несуществующей задачи → 404.
func TestTaskHandler_Delete_NotFound(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/999", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_Delete_NotTeamMember — actor не состоит в команде → 403.
func TestTaskHandler_Delete_NotTeamMember(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	stranger := e.users.Seed("s@example.com", "S")
	tok := tokenForID(t, stranger)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)
	e.tasks.SeedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: owner})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/1", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_Delete_BadID — id=0 или не-число → 400.
func TestTaskHandler_Delete_BadID(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/abc", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_Get_BadID_Zero — id=0 (валидное число, но недопустимое) → 400.
func TestTaskHandler_Get_BadID_Zero(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)
	w := doGet(e.router, "/api/v1/tasks/0", tok)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_Update_BadID — id=abc → 400.
func TestTaskHandler_Update_BadID(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	newTitle := "x"
	w := doJSON(e.router, http.MethodPut, "/api/v1/tasks/abc", tok, dto.UpdateTaskRequest{Title: &newTitle})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_Update_BadJSON — тело не JSON → 400.
func TestTaskHandler_Update_BadJSON(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/tasks/1", bytes.NewReader([]byte("{not-json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_List_NoTeamID — фильтр без team_id → 400 (TeamID==0 → ErrValidation).
func TestTaskHandler_List_NoTeamID(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/tasks", tok)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_List_BadStatusSilentlyIgnored(t *testing.T) {
	e := newHEnv(t)
	uid, _ := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/tasks?team_id=1&status=bogus", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 (silent ignore), got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_List_NotTeamMember — actor не в команде → 403.
func TestTaskHandler_List_NotTeamMember(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	stranger := e.users.Seed("s@example.com", "S")
	tok := tokenForID(t, stranger)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)

	w := doGet(e.router, "/api/v1/tasks?team_id=1", tok)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_History_NotTeamMember — 403 для не-членов команды.
func TestTaskHandler_History_NotTeamMember(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	stranger := e.users.Seed("s@example.com", "S")
	tok := tokenForID(t, stranger)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)
	e.tasks.SeedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: owner})

	w := doGet(e.router, "/api/v1/tasks/1/history", tok)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_History_BadID — id=abc → 400.
func TestTaskHandler_History_BadID(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/tasks/abc/history", tok)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_History_NotFound — задача не существует → 404.
func TestTaskHandler_History_NotFound(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/tasks/999/history", tok)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTeamHandler_Create_ActorNotInRepo(t *testing.T) {
	r := newBareHEnv(t).router

	// Токен выпускаем для uid=42, но в repo такого пользователя нет.
	tok := tokenForID(t, 42)

	w := doJSON(r, http.MethodPost, "/api/v1/teams", tok, dto.CreateTeamRequest{Name: "X"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTeamHandler_Create_LongName — name длиннее 100 символов → 400.
func TestTeamHandler_Create_LongName(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	long := strings.Repeat("a", 200)
	w := doJSON(e.router, http.MethodPost, "/api/v1/teams", tok, dto.CreateTeamRequest{Name: long})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTeamHandler_Create_BadJSON — тело не JSON → 400.
func TestTeamHandler_Create_BadJSON(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/teams", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTeamHandler_Invite_TeamNotFound — команды нет → 404.
func TestTeamHandler_Invite_TeamNotFound(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	target := e.users.Seed("t@example.com", "T")
	tok := tokenForID(t, owner)

	w := doJSON(e.router, http.MethodPost, "/api/v1/teams/999/invite", tok, dto.InviteRequest{
		UserID: target, Role: entity.RoleMember,
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTeamHandler_Invite_TargetUserNotFound — приглашаем несуществующего user'а → 404.
func TestTeamHandler_Invite_TargetUserNotFound(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	tok := tokenForID(t, owner)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)

	w := doJSON(e.router, http.MethodPost, "/api/v1/teams/1/invite", tok, dto.InviteRequest{
		UserID: 9999, Role: entity.RoleMember,
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTeamHandler_Invite_BadRole — role=invalid → 400 (service.ErrValidation).
func TestTeamHandler_Invite_BadRole(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	target := e.users.Seed("t@example.com", "T")
	tok := tokenForID(t, owner)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)

	w := doJSON(e.router, http.MethodPost, "/api/v1/teams/1/invite", tok, dto.InviteRequest{
		UserID: target, Role: entity.Role("bogus"),
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTeamHandler_Invite_BadID — id=abc → 400.
func TestTeamHandler_Invite_BadID(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doJSON(e.router, http.MethodPost, "/api/v1/teams/abc/invite", tok, dto.InviteRequest{
		UserID: 1, Role: entity.RoleMember,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTeamHandler_Invite_BadJSON — тело не JSON → 400.
func TestTeamHandler_Invite_BadJSON(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)
	e.teams.SeedTeam(1, "T", uid)
	e.teams.SeedMember(1, uid, entity.RoleOwner)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/teams/1/invite", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTeamHandler_ListMembers_BadID — id=abc → 400.
func TestTeamHandler_ListMembers_BadID(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/teams/abc/members", tok)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestCommentHandler_Create_NotFound — задача не существует → 404.
func TestCommentHandler_Create_NotFound(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doJSON(e.router, http.MethodPost, "/api/v1/tasks/999/comments", tok, dto.CreateCommentRequest{Body: "hi"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestCommentHandler_Create_NotTeamMember — actor не в команде задачи → 403.
func TestCommentHandler_Create_NotTeamMember(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	stranger := e.users.Seed("s@example.com", "S")
	tok := tokenForID(t, stranger)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)
	e.tasks.SeedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: owner})

	w := doJSON(e.router, http.MethodPost, "/api/v1/tasks/1/comments", tok, dto.CreateCommentRequest{Body: "hi"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestCommentHandler_Create_BadJSON — тело не JSON → 400.
func TestCommentHandler_Create_BadJSON(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)
	e.tasks.SeedTask(1, entity.Task{TeamID: teamID, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/1/comments", bytes.NewReader([]byte("{not-json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestCommentHandler_Create_BadID — id=abc → 400.
func TestCommentHandler_Create_BadID(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doJSON(e.router, http.MethodPost, "/api/v1/tasks/abc/comments", tok, dto.CreateCommentRequest{Body: "hi"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestCommentHandler_List_NotFound — задача не существует → 404.
func TestCommentHandler_List_NotFound(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/tasks/999/comments", tok)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestCommentHandler_List_BadID — id=abc → 400.
func TestCommentHandler_List_BadID(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/tasks/abc/comments", tok)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestCommentHandler_List_NotTeamMember — actor не в команде → 403.
func TestCommentHandler_List_NotTeamMember(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	stranger := e.users.Seed("s@example.com", "S")
	tok := tokenForID(t, stranger)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)
	e.tasks.SeedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: owner})

	w := doGet(e.router, "/api/v1/tasks/1/comments", tok)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestStatsHandler_LastWeek_EmptyItems(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/stats/teams/last-week", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w.Body.Bytes())
	v, exists := data["items"]
	if exists && v != nil {
		if items, ok := v.([]any); !ok || len(items) > 0 {
			t.Fatalf("items should be null/[], got %T(%v)", v, v)
		}
	}
}

func TestStatsHandler_TopCreators_DefaultValues(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/stats/top-creators", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestStatsHandler_TopCreators_BadQuery — невалидные числа → handler игнорирует
func TestStatsHandler_TopCreators_BadQuery(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/stats/top-creators?since_days=abc&limit=xyz", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestStatsHandler_TopCreators_NegativeQuery(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/stats/top-creators?since_days=-5&limit=0", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestStatsHandler_Orphans_Empty — пустой результат от fake'а → 200. items
func TestStatsHandler_Orphans_Empty(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doGet(e.router, "/api/v1/stats/orphan-tasks", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w.Body.Bytes())
	v, exists := data["items"]
	if exists && v != nil {
		if items, ok := v.([]any); !ok || len(items) > 0 {
			t.Fatalf("items should be null/[], got %T(%v)", v, v)
		}
	}
}

// ----- дополнительные edge-cases -----

func TestTaskHandler_Update_ClearAssignee(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)

	// AssigneeID = &0 → ClearAssignee=true → AssigneeID=nil.
	zero := uint64(0)
	e.tasks.SeedTask(1, entity.Task{
		TeamID:     teamID,
		Title:      "x",
		Status:     entity.TaskTodo,
		AssigneeID: &uid,
		CreatedBy:  uid,
	})

	w := doJSON(e.router, http.MethodPut, "/api/v1/tasks/1", tok,
		dto.UpdateTaskRequest{AssigneeID: &zero})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w.Body.Bytes())
	// В JSON assignee_id может быть null или отсутствовать — проверим, что
	// он больше не равен нашему uid.
	if v, ok := data["assignee_id"]; ok && v != nil {
		t.Fatalf("assignee_id should be nil, got %v", v)
	}
}

// TestTaskHandler_Update_InvalidStatus — status=invalid → buildPatchRich patchOK=false → 400.
func TestTaskHandler_Update_InvalidStatus(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)
	e.tasks.SeedTask(1, entity.Task{TeamID: teamID, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	bad := entity.TaskStatus("bogus")
	w := doJSON(e.router, http.MethodPut, "/api/v1/tasks/1", tok,
		dto.UpdateTaskRequest{Status: &bad})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_Create_InvalidStatus — status="" → ErrValidation → 400.
func TestTaskHandler_Create_InvalidStatus(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doJSON(e.router, http.MethodPost, "/api/v1/tasks", tok, dto.CreateTaskRequest{
		TeamID: teamID, Title: "x", Status: "",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_Create_EmptyTitle — title="" → ErrValidation → 400.
func TestTaskHandler_Create_EmptyTitle(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doJSON(e.router, http.MethodPost, "/api/v1/tasks", tok, dto.CreateTaskRequest{
		TeamID: teamID, Title: "   ", Status: entity.TaskTodo,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTaskHandler_Get_NotTeamMember — actor не в команде задачи → 403.
func TestTaskHandler_Get_NotTeamMember(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	stranger := e.users.Seed("s@example.com", "S")
	tok := tokenForID(t, stranger)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)
	e.tasks.SeedTask(1, entity.Task{TeamID: 1, Title: "x", Status: entity.TaskTodo, CreatedBy: owner})

	w := doGet(e.router, "/api/v1/tasks/1", tok)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestTeamHandler_Create_EmptyName — name="" → ErrValidation → 400.
func TestTeamHandler_Create_EmptyName(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doJSON(e.router, http.MethodPost, "/api/v1/teams", tok, dto.CreateTeamRequest{Name: ""})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTeamHandler_ListMembers_TeamMissing(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)
	// Команды 9999 нет — actor не в команде → ErrForbidden → 403.
	w := doGet(e.router, "/api/v1/teams/9999/members", tok)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

type authHEnv struct {
	router *gin.Engine
	svc    *service.AuthService
	users  *hFakeUserRepo
}

func newAuthHEnv(t *testing.T) *authHEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	users := newHFakeUserRepo()
	tm, err := auth.NewTokenManager(testkit.JWTConfigForTest())
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	hasher := auth.NewPasswordHasher(0)
	svc := service.NewAuthService(users, tm, hasher)

	r := gin.New()
	r.Use(gin.Recovery())
	authH := handler.NewAuthHandler(svc, nil)
	r.POST("/api/v1/register", authH.Register)
	r.POST("/api/v1/login", authH.Login)

	r.GET("/api/v1/protected",
		middleware.RequireAuth(tm, nil),
		func(c *gin.Context) {
			uid, _ := middleware.UserIDFromContext(c)
			c.JSON(http.StatusOK, gin.H{"uid": uid})
		},
	)
	return &authHEnv{router: r, svc: svc, users: users}
}

// TestAuthHandler_Register_EmailTaken_409 — дубликат email через реальный
func TestAuthHandler_Register_EmailTaken_409(t *testing.T) {
	e := newAuthHEnv(t)
	users := e.users
	users.Seed("x@example.com", "X") // уже существует

	w := postJSON(e.router, "/api/v1/register", "", dto.RegisterRequest{
		Email: "x@example.com", Password: "password123", Name: "X",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d body=%s", w.Code, w.Body.String())
	}
	env := errorFromEnvelope(t, w.Body.Bytes())
	if env["code"] != "conflict" {
		t.Fatalf("code: %v", env["code"])
	}
}

// TestAuthHandler_Login_UnknownUser_401_Code — явный assert на code=unauthorized.
func TestAuthHandler_Login_UnknownUser_401_Code(t *testing.T) {
	e := newAuthHEnv(t)
	w := postJSON(e.router, "/api/v1/login", "", dto.LoginRequest{
		Email: "nobody@example.com", Password: "any",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
	env := errorFromEnvelope(t, w.Body.Bytes())
	if env["code"] != "unauthorized" {
		t.Fatalf("code: %v", env["code"])
	}
}

// TestAuthHandler_Register_BadJSON — битый JSON → 400 (handler.ShouldBindJSON).
func TestAuthHandler_Register_BadJSON(t *testing.T) {
	e := newAuthHEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register",
		bytes.NewReader([]byte("{not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestAuthHandler_Login_BadJSON — битый JSON → 400.
func TestAuthHandler_Login_BadJSON(t *testing.T) {
	e := newAuthHEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login",
		bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// ----- helpers -----

func postJSON(r *gin.Engine, path, tok string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newBareHEnv(t *testing.T) *hEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	users := newHFakeUserRepo()
	teams := newHFakeTeamRepo()
	tasks := newHFakeTaskRepo()
	history := newHFakeHistoryRepo()
	comments := newHFakeCommentRepo()
	stats := newHFakeStatsRepo()
	cache := newHFakeCache()

	teamSvc := service.NewTeamService(teams, users, newHFakeTransactor())
	taskSvc := service.NewTaskService(tasks, history, teams, cache, newHFakeTransactor())
	commentSvc := service.NewCommentService(comments, tasks, teams, newHFakeTransactor())
	statsSvc := service.NewStatsService(stats)

	r := gin.New()
	r.Use(gin.Recovery())
	tm, err := auth.NewTokenManager(testkit.JWTConfigForTest())
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}

	teamH := handler.NewTeamHandler(teamSvc, nil)
	taskH := handler.NewTaskHandler(taskSvc, nil)
	commentH := handler.NewCommentHandler(commentSvc, nil)
	statsH := handler.NewStatsHandler(statsSvc, nil)

	g := r.Group("/api/v1")
	g.Use(middleware.RequireAuth(tm, nil))
	g.POST("/teams", teamH.Create)
	g.GET("/teams", teamH.List)
	g.POST("/teams/:id/invite", teamH.Invite)
	g.GET("/teams/:id/members", teamH.ListMembers)
	g.POST("/tasks", taskH.Create)
	g.GET("/tasks", taskH.List)
	g.GET("/tasks/:id", taskH.Get)
	g.PUT("/tasks/:id", taskH.Update)
	g.DELETE("/tasks/:id", taskH.Delete)
	g.GET("/tasks/:id/history", taskH.History)
	g.POST("/tasks/:id/comments", commentH.Create)
	g.GET("/tasks/:id/comments", commentH.List)
	g.GET("/stats/teams/last-week", statsH.LastWeek)
	g.GET("/stats/top-creators", statsH.TopCreators)
	g.GET("/stats/orphan-tasks", statsH.Orphans)

	return &hEnv{router: r, users: users, teams: teams, tasks: tasks}
}
