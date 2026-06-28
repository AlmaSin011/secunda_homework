package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/go-project/internal/auth"
	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/handler"
	"github.com/example/go-project/internal/middleware"
	"github.com/example/go-project/internal/service"
	"github.com/example/go-project/internal/testkit"
)

func tokenForID(t *testing.T, uid uint64) string {
	t.Helper()
	tm, err := auth.NewTokenManager(testkit.JWTConfigForTest())
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	tok, err := tm.Issue(uid, fmt.Sprintf("u%d@example.com", uid))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return tok
}

type hEnv struct {
	router *gin.Engine
	users  *hFakeUserRepo
	teams  *hFakeTeamRepo
	tasks  *hFakeTaskRepo
}

func newHEnv(t *testing.T) *hEnv {
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

	teamH := handler.NewTeamHandler(teamSvc, nil)
	taskH := handler.NewTaskHandler(taskSvc, nil)
	commentH := handler.NewCommentHandler(commentSvc, nil)
	statsH := handler.NewStatsHandler(statsSvc, nil)

	g := r.Group("/api/v1")
	g.Use(middleware.RequireAuth(tokenMgr(t), nil))
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

func tokenMgr(t *testing.T) *auth.TokenManager {
	t.Helper()
	tm, err := auth.NewTokenManager(testkit.JWTConfigForTest())
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	return tm
}

func doJSON(r *gin.Engine, method, path, tok string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func doGet(r *gin.Engine, path, tok string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func (e *hEnv) seedUserAndTeam(t *testing.T, email, name string) (uint64, uint64) {
	t.Helper()
	uid := e.users.Seed(email, name)
	teamID := uint64(1)
	e.teams.SeedTeam(teamID, "T", uid)
	e.teams.SeedMember(teamID, uid, entity.RoleOwner)
	return uid, teamID
}

func parseData(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var env struct {
		Data  map[string]any `json:"data"`
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v body=%s", err, string(body))
	}
	if env.Error != nil {
		t.Fatalf("unexpected error envelope: %+v body=%s", env.Error, string(body))
	}
	return env.Data
}

// ----- TEAM -----

func TestTeamHandler_Create_Success(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("alice@example.com", "Alice")
	tok := tokenForID(t, uid)

	w := doJSON(e.router, http.MethodPost, "/api/v1/teams", tok, dto.CreateTeamRequest{Name: "Backend"})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w.Body.Bytes())
	if data["name"] != "Backend" {
		t.Fatalf("name: %v", data["name"])
	}
	if data["my_role"] != "owner" {
		t.Fatalf("my_role: %v", data["my_role"])
	}
}

func TestTeamHandler_Create_ValidationError(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("alice@example.com", "Alice")
	tok := tokenForID(t, uid)

	w := doJSON(e.router, http.MethodPost, "/api/v1/teams", tok, dto.CreateTeamRequest{Name: "  "})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"validation_error"`) {
		t.Fatalf("want validation_error: %s", w.Body.String())
	}
}

func TestTeamHandler_List_OK(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)
	e.teams.SeedTeam(1, "T1", uid)
	e.teams.SeedMember(1, uid, entity.RoleOwner)
	e.teams.SeedTeam(2, "T2", uid)
	e.teams.SeedMember(2, uid, entity.RoleMember)

	w := doGet(e.router, "/api/v1/teams", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w.Body.Bytes())
	items, _ := data["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items len: got %d want 2", len(items))
	}
}

func TestTeamHandler_Invite_OK(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	newbie := e.users.Seed("n@example.com", "N")
	tok := tokenForID(t, owner)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)

	w := doJSON(e.router, http.MethodPost, "/api/v1/teams/1/invite", tok, dto.InviteRequest{
		UserID: newbie, Role: entity.RoleMember,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTeamHandler_Invite_NotOwner(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	member := e.users.Seed("m@example.com", "M")
	target := e.users.Seed("t@example.com", "T")
	tok := tokenForID(t, member)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)
	e.teams.SeedMember(1, member, entity.RoleMember)

	w := doJSON(e.router, http.MethodPost, "/api/v1/teams/1/invite", tok, dto.InviteRequest{
		UserID: target, Role: entity.RoleMember,
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTeamHandler_ListMembers_OK(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	tok := tokenForID(t, owner)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)
	e.teams.SeedMember(1, 42, entity.RoleMember)

	w := doGet(e.router, "/api/v1/teams/1/members", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w.Body.Bytes())
	items, _ := data["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("members len: got %d want 2", len(items))
	}
}

func TestTeamHandler_ListMembers_Forbidden(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	stranger := e.users.Seed("s@example.com", "S")
	tok := tokenForID(t, stranger)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)

	w := doGet(e.router, "/api/v1/teams/1/members", tok)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

// ----- TASK -----

func TestTaskHandler_Create_OK(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)

	w := doJSON(e.router, http.MethodPost, "/api/v1/tasks", tok, dto.CreateTaskRequest{
		TeamID: teamID, Title: "first", Status: entity.TaskTodo,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w.Body.Bytes())
	if data["title"] != "first" {
		t.Fatalf("title: %v", data["title"])
	}
}

func TestTaskHandler_Create_BadJSON(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader([]byte("{not-json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestTaskHandler_Create_NotTeamMember(t *testing.T) {
	e := newHEnv(t)
	owner := e.users.Seed("o@example.com", "O")
	stranger := e.users.Seed("s@example.com", "S")
	tok := tokenForID(t, stranger)
	e.teams.SeedTeam(1, "T", owner)
	e.teams.SeedMember(1, owner, entity.RoleOwner)

	w := doJSON(e.router, http.MethodPost, "/api/v1/tasks", tok, dto.CreateTaskRequest{
		TeamID: 1, Title: "x", Status: entity.TaskTodo,
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_List_OK(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)
	e.tasks.SeedTask(1, entity.Task{TeamID: teamID, Title: "a", Status: entity.TaskTodo, CreatedBy: uid})
	e.tasks.SeedTask(2, entity.Task{TeamID: teamID, Title: "b", Status: entity.TaskDone, CreatedBy: uid})

	w := doGet(e.router, "/api/v1/tasks?team_id="+fmt.Sprint(teamID), tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w.Body.Bytes())
	items, _ := data["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items len: got %d want 2", len(items))
	}
	meta, _ := data["meta"].(map[string]any)
	if meta["total"].(float64) != 2 {
		t.Fatalf("total: %v", meta["total"])
	}
}

func TestTaskHandler_Get_OK(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)
	e.tasks.SeedTask(1, entity.Task{TeamID: teamID, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	w := doGet(e.router, "/api/v1/tasks/1", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestTaskHandler_Get_NotFound(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)
	w := doGet(e.router, "/api/v1/tasks/999", tok)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_Get_BadID(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)
	w := doGet(e.router, "/api/v1/tasks/abc", tok)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestTaskHandler_Update_OK(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)
	e.tasks.SeedTask(1, entity.Task{TeamID: teamID, Title: "old", Status: entity.TaskTodo, CreatedBy: uid})

	newTitle := "new"
	w := doJSON(e.router, http.MethodPut, "/api/v1/tasks/1", tok, dto.UpdateTaskRequest{Title: &newTitle})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w.Body.Bytes())
	if data["title"] != "new" {
		t.Fatalf("title: %v", data["title"])
	}
}

func TestTaskHandler_Update_NoFields_BadRequest(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)
	e.tasks.SeedTask(1, entity.Task{TeamID: teamID, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	w := doJSON(e.router, http.MethodPut, "/api/v1/tasks/1", tok, dto.UpdateTaskRequest{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestTaskHandler_Delete_OK(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)
	e.tasks.SeedTask(1, entity.Task{TeamID: teamID, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/1", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_History_OK(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)
	e.tasks.SeedTask(1, entity.Task{TeamID: teamID, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	// история пустая → 200 c пустым массивом
	w := doGet(e.router, "/api/v1/tasks/1/history", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w.Body.Bytes())
	items, _ := data["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("items len: got %d want 0", len(items))
	}
}

// ----- COMMENT -----

func TestCommentHandler_Create_OK(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)
	e.tasks.SeedTask(1, entity.Task{TeamID: teamID, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	w := doJSON(e.router, http.MethodPost, "/api/v1/tasks/1/comments", tok, dto.CreateCommentRequest{Body: "hi"})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCommentHandler_Create_ValidationError(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)
	e.tasks.SeedTask(1, entity.Task{TeamID: teamID, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	w := doJSON(e.router, http.MethodPost, "/api/v1/tasks/1/comments", tok, dto.CreateCommentRequest{Body: "   "})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCommentHandler_List_OK(t *testing.T) {
	e := newHEnv(t)
	uid, teamID := e.seedUserAndTeam(t, "u@example.com", "U")
	tok := tokenForID(t, uid)
	e.tasks.SeedTask(1, entity.Task{TeamID: teamID, Title: "x", Status: entity.TaskTodo, CreatedBy: uid})

	w := doGet(e.router, "/api/v1/tasks/1/comments", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// ----- STATS -----

func TestStatsHandler_LastWeek_OK(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)
	w := doGet(e.router, "/api/v1/stats/teams/last-week", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestStatsHandler_TopCreators_OK(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)
	w := doGet(e.router, "/api/v1/stats/top-creators?since_days=14&limit=5", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestStatsHandler_Orphans_OK(t *testing.T) {
	e := newHEnv(t)
	uid := e.users.Seed("u@example.com", "U")
	tok := tokenForID(t, uid)
	w := doGet(e.router, "/api/v1/stats/orphan-tasks", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// ----- AUTH GATE -----

func TestProtected_NoToken(t *testing.T) {
	e := newHEnv(t)
	w := doGet(e.router, "/api/v1/teams", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestProtected_BadToken(t *testing.T) {
	e := newHEnv(t)
	w := doGet(e.router, "/api/v1/teams", "garbage")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

var _ = context.Background
var _ = time.Now
