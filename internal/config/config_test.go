package config

import (
	"testing"
	"time"
)

func TestLoad_WithEnv(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_SECRET", "test-secret-32-characters-long-xx")
	t.Setenv("MYSQL_DSN", "mysql://u:p@tcp(host:3306)/db?parseTime=true")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("JWT_TTL", "1h")
	t.Setenv("REDIS_TASKS_CACHE_TTL", "10m")
	t.Setenv("RATE_LIMIT_RPS", "2.5")
	t.Setenv("RATE_LIMIT_BURST", "50")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, "development")
	}
	if cfg.HTTP.Port != "9090" {
		t.Errorf("HTTP.Port = %q, want %q", cfg.HTTP.Port, "9090")
	}
	if cfg.JWT.Secret != "test-secret-32-characters-long-xx" {
		t.Errorf("JWT.Secret mismatch")
	}
	if cfg.JWT.TTL != time.Hour {
		t.Errorf("JWT.TTL = %v, want %v", cfg.JWT.TTL, time.Hour)
	}
	if cfg.Redis.TasksCacheTTL != 10*time.Minute {
		t.Errorf("Redis.TasksCacheTTL = %v, want %v", cfg.Redis.TasksCacheTTL, 10*time.Minute)
	}
	if cfg.RateLimit.RPS != 2.5 {
		t.Errorf("RateLimit.RPS = %v, want %v", cfg.RateLimit.RPS, 2.5)
	}
	if cfg.RateLimit.Burst != 50 {
		t.Errorf("RateLimit.Burst = %d, want 50", cfg.RateLimit.Burst)
	}
	if cfg.MySQL.DSN == "" {
		t.Errorf("MySQL.DSN is empty")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_SECRET", "x") // валидация на длину только в production

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.HTTP.Port != "8080" {
		t.Errorf("default HTTP.Port = %q, want %q", cfg.HTTP.Port, "8080")
	}
	if cfg.Log.Level != "info" {
		t.Errorf("default Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
	if cfg.JWT.TTL != 24*time.Hour {
		t.Errorf("default JWT.TTL = %v, want %v", cfg.JWT.TTL, 24*time.Hour)
	}
	if cfg.JWT.Issuer != "taskmgr" {
		t.Errorf("default JWT.Issuer = %q, want %q", cfg.JWT.Issuer, "taskmgr")
	}
	if cfg.Redis.DB != 0 {
		t.Errorf("default Redis.DB = %d, want 0", cfg.Redis.DB)
	}
	if cfg.Metrics.Path != "/metrics" {
		t.Errorf("default Metrics.Path = %q, want /metrics", cfg.Metrics.Path)
	}
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	t.Setenv("APP_ENV", "development")

	t.Setenv("JWT_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing JWT_SECRET, got nil")
	}
}

func TestLoad_ProductionRequiresLongSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "short")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for short JWT_SECRET in production, got nil")
	}
}

func TestLoad_ProductionAcceptsLongSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "very-long-production-secret-32-chars-min")

	_, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
}

func TestLoad_InvalidRateLimit(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_SECRET", "x")
	t.Setenv("RATE_LIMIT_RPS", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for RATE_LIMIT_RPS=0, got nil")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid dev",
			cfg:     Config{AppEnv: "development", JWT: JWTConfig{Secret: "x", TTL: time.Hour}, HTTP: HTTPConfig{Port: "8080"}, RateLimit: RateLimitConfig{RPS: 1, Burst: 10}},
			wantErr: false,
		},
		{
			name:    "empty secret",
			cfg:     Config{AppEnv: "development", JWT: JWTConfig{Secret: "", TTL: time.Hour}, HTTP: HTTPConfig{Port: "8080"}, RateLimit: RateLimitConfig{RPS: 1, Burst: 10}},
			wantErr: true,
		},
		{
			name:    "short prod secret",
			cfg:     Config{AppEnv: "production", JWT: JWTConfig{Secret: "short", TTL: time.Hour}, HTTP: HTTPConfig{Port: "8080"}, RateLimit: RateLimitConfig{RPS: 1, Burst: 10}},
			wantErr: true,
		},
		{
			name:    "negative TTL",
			cfg:     Config{AppEnv: "development", JWT: JWTConfig{Secret: "x", TTL: -time.Hour}, HTTP: HTTPConfig{Port: "8080"}, RateLimit: RateLimitConfig{RPS: 1, Burst: 10}},
			wantErr: true,
		},
		{
			name:    "empty port",
			cfg:     Config{AppEnv: "development", JWT: JWTConfig{Secret: "x", TTL: time.Hour}, HTTP: HTTPConfig{Port: ""}, RateLimit: RateLimitConfig{RPS: 1, Burst: 10}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
