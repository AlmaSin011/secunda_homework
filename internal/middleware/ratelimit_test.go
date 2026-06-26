package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/example/go-project/internal/auth"
	"github.com/example/go-project/internal/middleware"
	"github.com/example/go-project/internal/utills"
)

func newTestRouter(tm *auth.TokenManager, rps float64, burst int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	limiter := middleware.NewPerUserLimiter(rps, burst)

	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	g := r.Group("/api")
	g.Use(middleware.RequireAuth(tm, nil))
	g.Use(middleware.RateLimit(limiter))
	g.GET("/me", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func TestRateLimit_AllowsBelowBurst(t *testing.T) {
	tm, err := auth.NewTokenManager(utills.JWTConfigForTest())
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	// burst=3, rps маленький — первые 3 запроса пройдут, 4-й — 429.
	r := newTestRouter(tm, 0.01, 3)
	tok, _ := tm.Issue(42, "u@example.com")

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: want 200, got %d", i, w.Code)
		}
	}
}

func TestRateLimit_BlocksAboveBurst(t *testing.T) {
	tm, err := auth.NewTokenManager(utills.JWTConfigForTest())
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	r := newTestRouter(tm, 0.01, 2)
	tok, _ := tm.Issue(7, "a@example.com")

	// первые 2 — ок
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("warm-up %d: want 200, got %d", i, w.Code)
		}
	}
	// 3-й — лимит
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("over-burst: want 429, got %d", w.Code)
	}
}

func TestRateLimit_PerUserIsolation(t *testing.T) {
	tm, err := auth.NewTokenManager(utills.JWTConfigForTest())
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	// burst=1 на пользователя: каждый имеет свой bucket.
	r := newTestRouter(tm, 0.01, 1)
	tok1, _ := tm.Issue(1, "a@example.com")
	tok2, _ := tm.Issue(2, "b@example.com")

	// user 1: первый — ок, второй — 429
	w := httptest.NewRecorder()
	r.ServeHTTP(w, withBearer("GET", "/api/me", tok1))
	if w.Code != http.StatusOK {
		t.Fatalf("u1 first: want 200, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, withBearer("GET", "/api/me", tok1))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("u1 second: want 429, got %d", w.Code)
	}
	// user 2: должен пройти, bucket изолирован
	w = httptest.NewRecorder()
	r.ServeHTTP(w, withBearer("GET", "/api/me", tok2))
	if w.Code != http.StatusOK {
		t.Fatalf("u2 first: want 200, got %d", w.Code)
	}
}

func withBearer(method, path, tok string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	return req
}
