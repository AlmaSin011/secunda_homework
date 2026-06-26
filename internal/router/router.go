package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/example/go-project/internal/auth"
	"github.com/example/go-project/internal/handler"
	"github.com/example/go-project/internal/middleware"
)

type Deps struct {
	Logger      *slog.Logger
	TokenMgr    *auth.TokenManager
	Limiter     *middleware.PerUserLimiter
	Auth        *handler.AuthHandler
	Teams       *handler.TeamHandler
	Tasks       *handler.TaskHandler
	Comments    *handler.CommentHandler
	Stats       *handler.StatsHandler
	MetricsPath string
}

// /healthz, /metrics, /api/v1/register, /api/v1/login — без авторизации.
// Всё остальное под /api/v1 — RequireAuth + RateLimit.
func New(d Deps) *gin.Engine {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger(d.Logger))
	r.Use(middleware.Prometheus())

	// --- public ---
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	if d.MetricsPath != "" {
		r.GET(d.MetricsPath, gin.WrapH(promhttp.Handler()))
	}
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// --- v1 API ---
	v1 := r.Group("/api/v1")
	v1.POST("/register", d.Auth.Register)
	v1.POST("/login", d.Auth.Login)

	authed := v1.Group("")
	authed.Use(middleware.RequireAuth(d.TokenMgr, d.Logger))
	authed.Use(middleware.RateLimit(d.Limiter))

	// Teams
	authed.POST("/teams", d.Teams.Create)
	authed.GET("/teams", d.Teams.List)
	authed.POST("/teams/:id/invite", d.Teams.Invite)
	authed.GET("/teams/:id/members", d.Teams.ListMembers)

	// Tasks
	authed.POST("/tasks", d.Tasks.Create)
	authed.GET("/tasks", d.Tasks.List)
	authed.GET("/tasks/:id", d.Tasks.Get)
	authed.PUT("/tasks/:id", d.Tasks.Update)
	authed.DELETE("/tasks/:id", d.Tasks.Delete)
	authed.GET("/tasks/:id/history", d.Tasks.History)

	// Comments
	authed.POST("/tasks/:id/comments", d.Comments.Create)
	authed.GET("/tasks/:id/comments", d.Comments.List)

	// Stats
	authed.GET("/stats/teams/last-week", d.Stats.LastWeek)
	authed.GET("/stats/top-creators", d.Stats.TopCreators)
	authed.GET("/stats/orphan-tasks", d.Stats.Orphans)

	return r
}

func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http request",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
		)
	}
}
