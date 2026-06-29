//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/example/go-project/internal/auth"
	"github.com/example/go-project/internal/handler"
	"github.com/example/go-project/internal/repository"
	"github.com/example/go-project/internal/router"
	"github.com/example/go-project/internal/service"
	"github.com/example/go-project/internal/testkit"

	"github.com/jmoiron/sqlx"

	_ "github.com/go-sql-driver/mysql"
)

type httpHarness struct {
	srv      *httptest.Server
	db       *sqlx.DB
	tokenMgr *auth.TokenManager
	pw       *auth.PasswordHasher
}

func newHTTPHarness(t *testing.T) *httpHarness {
	t.Helper()

	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_TEST_DSN")
	}
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not set, skipping HTTP e2e integration test")
	}

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		t.Skipf("MySQL not reachable: %v", err)
	}
	db.SetMaxOpenConns(5)

	cleanup(t, db)
	t.Cleanup(func() {
		cleanup(t, db)
		_ = db.Close()
	})

	tm, err := auth.NewTokenManager(testkit.JWTConfigForTest())
	if err != nil {
		t.Fatalf("TokenManager: %v", err)
	}
	pw := auth.NewPasswordHasher(bcryptMin)

	userRepo := repository.NewUserRepository(db)
	teamRepo := repository.NewTeamRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	histRepo := repository.NewHistoryRepository(db)
	commentRepo := repository.NewCommentRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	tx := service.NewSQLXTransactor(db)

	authSvc := service.NewAuthService(userRepo, tm, pw)
	teamSvc := service.NewTeamService(teamRepo, userRepo, tx)
	taskSvc := service.NewTaskService(taskRepo, histRepo, teamRepo, nil, tx)
	commentSvc := service.NewCommentService(commentRepo, taskRepo, teamRepo, tx)
	statsSvc := service.NewStatsService(statsRepo)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := router.New(router.Deps{
		Logger:      logger,
		TokenMgr:    tm,
		Limiter:     nil,
		Auth:        handler.NewAuthHandler(authSvc, logger),
		Teams:       handler.NewTeamHandler(teamSvc, logger),
		Tasks:       handler.NewTaskHandler(taskSvc, logger),
		Comments:    handler.NewCommentHandler(commentSvc, logger),
		Stats:       handler.NewStatsHandler(statsSvc, logger),
		MetricsPath: "",
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return &httpHarness{
		srv:      srv,
		db:       db,
		tokenMgr: tm,
		pw:       pw,
	}
}

const bcryptMin = 4.

type httpResp struct {
	status int
	body   map[string]any
	raw    []byte
}

func (h *httpHarness) do(t *testing.T, method, path string, token string, body any) httpResp {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, h.srv.URL+path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := httpResp{status: resp.StatusCode, raw: raw}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.body)
	}
	return out
}

func (h *httpHarness) register(t *testing.T, email, name string) (token string, userID uint64, status int) {
	t.Helper()
	resp := h.do(t, http.MethodPost, "/api/v1/register", "", map[string]any{
		"email":    email,
		"password": "Pa55word!",
		"name":     name,
	})
	if resp.status != http.StatusCreated {
		t.Fatalf("register: status=%d body=%s", resp.status, resp.raw)
	}
	data, _ := resp.body["data"].(map[string]any)
	tok, _ := data["token"].(string)
	userObj, _ := data["user"].(map[string]any)
	idF, _ := userObj["id"].(float64)
	return tok, uint64(idF), resp.status
}

func (h *httpHarness) login(t *testing.T, email string) string {
	t.Helper()
	resp := h.do(t, http.MethodPost, "/api/v1/login", "", map[string]any{
		"email":    email,
		"password": "Pa55word!",
	})
	if resp.status != http.StatusOK {
		t.Fatalf("login: status=%d body=%s", resp.status, resp.raw)
	}
	data, _ := resp.body["data"].(map[string]any)
	tok, _ := data["token"].(string)
	return tok
}

func newEmail(suffix string) string {
	return fmt.Sprintf("user-%s-%d@example.com", suffix, time.Now().UnixNano())
}

func TestHTTP_AuthRegisterLogin(t *testing.T) {
	h := newHTTPHarness(t)

	email := newEmail("reg")

	// 1. Регистрация
	tok1, uid, status := h.register(t, email, "Alice")
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", status)
	}
	if tok1 == "" || uid == 0 {
		t.Fatalf("missing token or user id")
	}

	// 409
	dup := h.do(t, http.MethodPost, "/api/v1/register", "", map[string]any{
		"email":    email,
		"password": "Pa55word!",
		"name":     "Alice",
	})
	if dup.status != http.StatusConflict {
		t.Errorf("duplicate register expected 409, got %d", dup.status)
	}

	tok2 := h.login(t, email)
	if tok2 == "" {
		t.Fatal("login returned empty token")
	}

	bad := h.do(t, http.MethodPost, "/api/v1/login", "", map[string]any{
		"email":    email,
		"password": "wrong-password",
	})
	if bad.status != http.StatusUnauthorized {
		t.Errorf("wrong password expected 401, got %d", bad.status)
	}

	badReq := h.do(t, http.MethodPost, "/api/v1/register", "", map[string]any{
		"email":    "",
		"password": "shortpw",
		"name":     "",
	})
	if badReq.status != http.StatusBadRequest {
		t.Errorf("bad request expected 400, got %d", badReq.status)
	}
}

func TestHTTP_TeamAndMemberFlow(t *testing.T) {
	h := newHTTPHarness(t)

	ownerTok, _, _ := h.register(t, newEmail("owner"), "Owner")
	bobEmail := newEmail("bob")
	bobTok, bobID, _ := h.register(t, bobEmail, "Bob")
	_ = bobTok

	create := h.do(t, http.MethodPost, "/api/v1/teams", ownerTok, map[string]any{
		"name": "Integration Team",
	})
	if create.status != http.StatusCreated {
		t.Fatalf("create team: status=%d body=%s", create.status, create.raw)
	}
	data, _ := create.body["data"].(map[string]any)
	teamIDF, _ := data["id"].(float64)
	teamID := uint64(teamIDF)
	if teamID == 0 {
		t.Fatal("team id is zero")
	}

	invite := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/v1/teams/%d/invite", teamID),
		ownerTok,
		map[string]any{"user_id": bobID, "role": "member"},
	)
	if invite.status != http.StatusCreated {
		t.Fatalf("invite: status=%d body=%s", invite.status, invite.raw)
	}

	listForBob := h.do(t, http.MethodGet, "/api/v1/teams", bobTok, nil)
	if listForBob.status != http.StatusOK {
		t.Fatalf("list: status=%d", listForBob.status)
	}
	data, _ = listForBob.body["data"].(map[string]any)
	items, _ := data["items"].([]any)
	if len(items) != 1 {
		t.Errorf("bob expected 1 team, got %d", len(items))
	}

	_, _, _ = h.register(t, newEmail("carol"), "Carol")
	members := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/v1/teams/%d/members", teamID), ownerTok, nil)
	if members.status != http.StatusOK {
		t.Fatalf("list members: status=%d", members.status)
	}
	data, _ = members.body["data"].(map[string]any)
	list, _ := data["items"].([]any)
	if len(list) != 2 {
		t.Errorf("expected 2 members (owner+invited), got %d", len(list))
	}

	// Дубль инвайта
	dupInvite := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/v1/teams/%d/invite", teamID),
		ownerTok,
		map[string]any{"user_id": bobID, "role": "member"},
	)
	if dupInvite.status != http.StatusConflict {
		t.Errorf("dup invite expected 409, got %d", dupInvite.status)
	}
}

func TestHTTP_TaskCRUDFlow(t *testing.T) {
	h := newHTTPHarness(t)
	ownerTok, _, _ := h.register(t, newEmail("task-owner"), "Owner")

	// Создаём команду через owner.
	create := h.do(t, http.MethodPost, "/api/v1/teams", ownerTok, map[string]any{
		"name": "Task Team",
	})
	if create.status != http.StatusCreated {
		t.Fatalf("create team: %d %s", create.status, create.raw)
	}
	data, _ := create.body["data"].(map[string]any)
	teamIDF, _ := data["id"].(float64)
	teamID := uint64(teamIDF)

	// Создаём задачу.
	taskCreate := h.do(t, http.MethodPost, "/api/v1/tasks", ownerTok, map[string]any{
		"team_id": teamID,
		"title":   "Initial title",
		"status":  "todo",
	})
	if taskCreate.status != http.StatusCreated {
		t.Fatalf("create task: %d %s", taskCreate.status, taskCreate.raw)
	}
	data, _ = taskCreate.body["data"].(map[string]any)
	taskIDF, _ := data["id"].(float64)
	taskID := uint64(taskIDF)
	if taskID == 0 {
		t.Fatal("task id is zero")
	}

	get := h.do(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", taskID), ownerTok, nil)
	if get.status != http.StatusOK {
		t.Fatalf("get task: %d", get.status)
	}

	list := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/v1/tasks?team_id=%d&limit=10", teamID),
		ownerTok,
		nil,
	)
	if list.status != http.StatusOK {
		t.Fatalf("list: %d", list.status)
	}
	data, _ = list.body["data"].(map[string]any)
	items, _ := data["items"].([]any)
	if len(items) != 1 {
		t.Errorf("expected 1 task in list, got %d", len(items))
	}

	upd := h.do(t, http.MethodPut,
		fmt.Sprintf("/api/v1/tasks/%d", taskID),
		ownerTok,
		map[string]any{
			"title":  "Updated title",
			"status": "in_progress",
		},
	)
	if upd.status != http.StatusOK {
		t.Fatalf("update: %d %s", upd.status, upd.raw)
	}

	// История должна содержать 2 записи: title, status.
	hist := h.do(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/history", taskID), ownerTok, nil)
	if hist.status != http.StatusOK {
		t.Fatalf("history: %d", hist.status)
	}
	data, _ = hist.body["data"].(map[string]any)
	hrows, _ := data["items"].([]any)
	if len(hrows) != 2 {
		t.Errorf("expected 2 history rows, got %d: %v", len(hrows), hrows)
	}

	//  update без полей - 400.
	noOp := h.do(t, http.MethodPut,
		fmt.Sprintf("/api/v1/tasks/%d", taskID),
		ownerTok, map[string]any{},
	)
	if noOp.status != http.StatusBadRequest {
		t.Errorf("empty patch expected 400, got %d", noOp.status)
	}

	// Soft delete → 204.
	del := h.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", taskID), ownerTok, nil)
	if del.status != http.StatusNoContent {
		t.Errorf("delete: status=%d", del.status)
	}

	listAfter := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/v1/tasks?team_id=%d", teamID), ownerTok, nil)
	data, _ = listAfter.body["data"].(map[string]any)
	items, _ = data["items"].([]any)
	if len(items) != 0 {
		t.Errorf("expected empty list after soft-delete, got %d", len(items))
	}
}

func TestHTTP_CommentFlow(t *testing.T) {
	h := newHTTPHarness(t)
	ownerTok, _, _ := h.register(t, newEmail("c-owner"), "Owner")
	strangerEmail := newEmail("c-stranger")
	strangerTok, _, _ := h.register(t, strangerEmail, "Stranger")

	create := h.do(t, http.MethodPost, "/api/v1/teams", ownerTok, map[string]any{
		"name": "Comment Team",
	})
	data, _ := create.body["data"].(map[string]any)
	teamID := uint64(data["id"].(float64))

	tCreate := h.do(t, http.MethodPost, "/api/v1/tasks", ownerTok, map[string]any{
		"team_id": teamID,
		"title":   "Commented task",
		"status":  "todo",
	})
	if tCreate.status != http.StatusCreated {
		t.Fatalf("create task: %d", tCreate.status)
	}
	data, _ = tCreate.body["data"].(map[string]any)
	taskID := uint64(data["id"].(float64))

	c1 := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/v1/tasks/%d/comments", taskID),
		ownerTok,
		map[string]any{"body": "first!"},
	)
	if c1.status != http.StatusCreated {
		t.Fatalf("create comment: %d %s", c1.status, c1.raw)
	}
	data, _ = c1.body["data"].(map[string]any)
	if _, ok := data["id"]; !ok {
		t.Fatalf("comment id missing: %+v", data)
	}

	cDenied := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/v1/tasks/%d/comments", taskID),
		strangerTok,
		map[string]any{"body": "intruder"},
	)
	if cDenied.status != http.StatusForbidden {
		t.Errorf("stranger comment expected 403, got %d body=%s", cDenied.status, cDenied.raw)
	}

	listDenied := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/v1/tasks/%d/comments", taskID),
		strangerTok,
		nil,
	)
	if listDenied.status != http.StatusForbidden {
		t.Errorf("stranger list comments expected 403, got %d", listDenied.status)
	}

	// Владелец читает — список непустой.
	listOK := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/v1/tasks/%d/comments", taskID),
		ownerTok,
		nil,
	)
	if listOK.status != http.StatusOK {
		t.Fatalf("owner list comments: %d", listOK.status)
	}
	data, _ = listOK.body["data"].(map[string]any)
	items, _ := data["items"].([]any)
	if len(items) != 1 {
		t.Errorf("expected 1 comment, got %d", len(items))
	}

	emptyBody := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/v1/tasks/%d/comments", taskID),
		ownerTok,
		map[string]any{"body": "   "},
	)
	if emptyBody.status != http.StatusBadRequest {
		t.Errorf("empty comment body expected 400, got %d", emptyBody.status)
	}
}

func TestHTTP_StatsEndpoints(t *testing.T) {
	h := newHTTPHarness(t)
	ownerTok, _, _ := h.register(t, newEmail("stats-owner"), "Owner")
	bobEmail := newEmail("bob-stats")
	bobTok, _, _ := h.register(t, bobEmail, "Bob")

	// Команда.
	create := h.do(t, http.MethodPost, "/api/v1/teams", ownerTok, map[string]any{
		"name": "Stats Team",
	})
	data, _ := create.body["data"].(map[string]any)
	teamID := uint64(data["id"].(float64))

	_ = h.do(t, http.MethodPost,
		fmt.Sprintf("/api/v1/teams/%d/invite", teamID), ownerTok,
		map[string]any{"user_id": bobIDFromToken(t, h, bobTok), "role": "member"},
	)

	// Несколько задач.
	makeTask := func(title, status, tok string) {
		r := h.do(t, http.MethodPost, "/api/v1/tasks", tok, map[string]any{
			"team_id": teamID,
			"title":   title,
			"status":  status,
		})
		if r.status != http.StatusCreated {
			t.Fatalf("create task %q: %d %s", title, r.status, r.raw)
		}
	}
	makeTask("Todo 1", "todo", ownerTok)
	makeTask("Todo 2", "todo", bobTok)
	makeTask("Done 1", "in_progress", ownerTok)

	doneID := taskIDByTitle(t, h, ownerTok, teamID, "Todo 1")
	d := h.do(t, http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", doneID), ownerTok,
		map[string]any{"status": "done"})
	if d.status != http.StatusOK {
		t.Fatalf("update done: %d %s", d.status, d.raw)
	}

	// last-week.
	lastWeek := h.do(t, http.MethodGet, "/api/v1/stats/teams/last-week", ownerTok, nil)
	if lastWeek.status != http.StatusOK {
		t.Fatalf("last-week: %d %s", lastWeek.status, lastWeek.raw)
	}
	data, _ = lastWeek.body["data"].(map[string]any)
	items, _ := data["items"].([]any)
	if len(items) < 1 {
		t.Errorf("expected at least 1 team in last-week stats, got %d", len(items))
	}

	// top-creators.
	top := h.do(t, http.MethodGet, "/api/v1/stats/top-creators?since_days=30&limit=3", ownerTok, nil)
	if top.status != http.StatusOK {
		t.Fatalf("top-creators: %d %s", top.status, top.raw)
	}

	// orphan-tasks.
	orphans := h.do(t, http.MethodGet, "/api/v1/stats/orphan-tasks", ownerTok, nil)
	if orphans.status != http.StatusOK {
		t.Fatalf("orphan-tasks: %d %s", orphans.status, orphans.raw)
	}
	data, _ = orphans.body["data"].(map[string]any)
	_ = data
}

func TestHTTP_UnauthorizedAndForbidden(t *testing.T) {
	h := newHTTPHarness(t)
	tok, _, _ := h.register(t, newEmail("u"), "User")

	// Анонимный → 401.
	anon := h.do(t, http.MethodGet, "/api/v1/teams", "", nil)
	if anon.status != http.StatusUnauthorized {
		t.Errorf("anon expected 401, got %d", anon.status)
	}

	// Битый токен → 401.
	badTok := h.do(t, http.MethodGet, "/api/v1/teams", "not-a-token", nil)
	if badTok.status != http.StatusUnauthorized {
		t.Errorf("bad token expected 401, got %d", badTok.status)
	}

	create := h.do(t, http.MethodPost, "/api/v1/teams", tok, map[string]any{
		"name": "User Team",
	})
	data, _ := create.body["data"].(map[string]any)
	teamID := uint64(data["id"].(float64))

	listing := h.do(t, http.MethodGet, fmt.Sprintf("/api/v1/teams/%d/members", teamID), tok, nil)
	if listing.status != http.StatusOK {
		t.Fatalf("owner should see members: %d", listing.status)
	}

	strangerEmail := newEmail("str")
	strangerTok, _, _ := h.register(t, strangerEmail, "Stranger")
	denied := h.do(t, http.MethodGet, fmt.Sprintf("/api/v1/teams/%d/members", teamID), strangerTok, nil)
	if denied.status != http.StatusForbidden {
		t.Errorf("stranger member-list expected 403, got %d", denied.status)
	}
}

func TestHTTP_HealthzNoAuth(t *testing.T) {
	h := newHTTPHarness(t)

	for _, path := range []string{"/healthz", "/ping"} {
		r := h.do(t, http.MethodGet, path, "", nil)
		if r.status != http.StatusOK {
			t.Errorf("%s expected 200, got %d body=%s", path, r.status, r.raw)
		}
	}
}

func bobIDFromToken(t *testing.T, h *httpHarness, tok string) uint64 {
	t.Helper()
	claims, err := h.tokenMgr.Parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return claims.UserID()
}

func taskIDByTitle(t *testing.T, h *httpHarness, tok string, teamID uint64, title string) uint64 {
	t.Helper()
	url := fmt.Sprintf("/api/v1/tasks?team_id=%d&limit=100", teamID)
	r := h.do(t, http.MethodGet, url, tok, nil)
	if r.status != http.StatusOK {
		t.Fatalf("list: %d %s", r.status, r.raw)
	}
	data, _ := r.body["data"].(map[string]any)
	items, _ := data["items"].([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		if strings.EqualFold(asString(m["title"]), title) {
			idF, _ := m["id"].(float64)
			return uint64(idF)
		}
	}
	t.Fatalf("task %q not found in list", title)
	return 0
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
