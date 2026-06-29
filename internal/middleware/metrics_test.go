package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/example/go-project/internal/middleware"
)

func TestPrometheus_RecordsRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Prometheus())

	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	r.GET("/fail", func(c *gin.Context) { c.String(http.StatusBadRequest, "bad") })

	// Успешный запрос
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// httpErrorsTotal
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/fail", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w2.Code)
	}

	// Запрос без маршрута (для unknown path)
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/not-registered", nil)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w3.Code)
	}
}

func TestPrometheus_EmptyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Prometheus())

	r.NoRoute(func(c *gin.Context) { c.String(http.StatusNotFound, "404") })

	// Запрос на несуществующий путь → path = "" в c.FullPath()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestRateLimit_FallbackForUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	limiter := middleware.NewPerUserLimiter(0.01, 1)
	g := r.Group("/api")
	g.Use(middleware.RateLimit(limiter)) // БЕЗ RequireAuth → uid=0 → fallback
	g.GET("/anon", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// 1-й — OK (burst=1), 2-й — 429 (uid=0 → fallback bucket).
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/anon", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("anon first: want 200, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/anon", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("anon over: want 429, got %d", w.Code)
	}
}
