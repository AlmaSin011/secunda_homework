// handlers_unauth_test.go — coverage для веток handler-ов, которые срабатывают
// только когда middleware.RequireAuth НЕ выставил uid в контекст.
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/example/go-project/internal/handler"
	"github.com/example/go-project/internal/service"
)

type noAuthEnv struct {
	router *gin.Engine
}

func newNoAuthEnv(t *testing.T) *noAuthEnv {
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
	// БЕЗ RequireAuth: контекст не содержит uid → ветка `if !ok` сработает.
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

	return &noAuthEnv{router: r}
}

func TestTeamHandler_NoUID(t *testing.T) {
	e := newNoAuthEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/teams", nil)
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTeamHandler_NoUID_List(t *testing.T) {
	e := newNoAuthEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/teams", nil)
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTeamHandler_NoUID_Invite(t *testing.T) {
	e := newNoAuthEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/teams/1/invite", nil)
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTeamHandler_NoUID_ListMembers(t *testing.T) {
	e := newNoAuthEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/teams/1/members", nil)
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_NoUID_Create(t *testing.T) {
	e := newNoAuthEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", nil)
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_NoUID_Get(t *testing.T) {
	e := newNoAuthEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/1", nil)
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_NoUID_Update(t *testing.T) {
	e := newNoAuthEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/tasks/1", nil)
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_NoUID_Delete(t *testing.T) {
	e := newNoAuthEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/1", nil)
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_NoUID_History(t *testing.T) {
	e := newNoAuthEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/1/history", nil)
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_NoUID_List(t *testing.T) {
	e := newNoAuthEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCommentHandler_NoUID_Create(t *testing.T) {
	e := newNoAuthEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/1/comments", nil)
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCommentHandler_NoUID_List(t *testing.T) {
	e := newNoAuthEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/1/comments", nil)
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}
