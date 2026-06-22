package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/go-project/internal/cache"
	"github.com/example/go-project/internal/config"
	"github.com/example/go-project/internal/storage"
	"github.com/example/go-project/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// --- Config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(cfg.Log.Level)
	slog.SetDefault(log)
	log.Info("starting service",
		slog.String("env", cfg.AppEnv),
		slog.String("port", cfg.HTTP.Port),
	)

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	db, err := storage.OpenMySQL(rootCtx, storage.MySQLConfig{
		DSN:             cfg.MySQL.DSN,
		MaxOpenConns:    cfg.MySQL.MaxOpenConns,
		MaxIdleConns:    cfg.MySQL.MaxIdleConns,
		ConnMaxLifetime: cfg.MySQL.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.MySQL.ConnMaxIdleTime,
	})
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Warn("mysql close", slog.String("err", err.Error()))
		}
	}()
	log.Info("mysql connected")

	rdb, err := cache.NewRedisCache(rootCtx, cfg.Redis)
	if err != nil {
		return fmt.Errorf("open redis: %w", err)
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Warn("redis close", slog.String("err", err.Error()))
		}
	}()
	log.Info("redis connected")

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), requestLogger(log))

	r.GET("/healthz", func(c *gin.Context) {
		pingCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(pingCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_down", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// --- Server + graceful shutdown -------------------------------------
	srv := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Info("shutdown signal received", slog.String("signal", sig.String()))
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}
	log.Info("server stopped")
	return nil
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
