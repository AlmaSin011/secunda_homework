package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv    string `mapstructure:"app_env"`
	HTTP      HTTPConfig
	Log       LogConfig
	MySQL     MySQLConfig
	Redis     RedisConfig
	JWT       JWTConfig
	RateLimit RateLimitConfig
	Metrics   MetricsConfig
}

type HTTPConfig struct {
	Port string `mapstructure:"port"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type MySQLConfig struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

type RedisConfig struct {
	Addr          string        `mapstructure:"addr"`
	Password      string        `mapstructure:"password"`
	DB            int           `mapstructure:"db"`
	PoolSize      int           `mapstructure:"pool_size"`
	DialTimeout   time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout   time.Duration `mapstructure:"read_timeout"`
	WriteTimeout  time.Duration `mapstructure:"write_timeout"`
	TasksCacheTTL time.Duration `mapstructure:"tasks_cache_ttl"`
}

type JWTConfig struct {
	Secret string        `mapstructure:"secret"`
	TTL    time.Duration `mapstructure:"ttl"`
	Issuer string        `mapstructure:"issuer"`
}

type RateLimitConfig struct {
	RPS   float64 `mapstructure:"rps"`
	Burst int     `mapstructure:"burst"`
}

type MetricsConfig struct {
	Path string `mapstructure:"path"`
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetDefault("app_env", "development")
	v.SetDefault("http.port", "8080")
	v.SetDefault("log.level", "info")

	v.SetDefault("mysql.max_open_conns", 25)
	v.SetDefault("mysql.max_idle_conns", 5)
	v.SetDefault("mysql.conn_max_lifetime", "5m")
	v.SetDefault("mysql.conn_max_idle_time", "2m")

	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("redis.dial_timeout", "2s")
	v.SetDefault("redis.read_timeout", "500ms")
	v.SetDefault("redis.write_timeout", "500ms")
	v.SetDefault("redis.tasks_cache_ttl", "5m")

	v.SetDefault("jwt.ttl", "24h")
	v.SetDefault("jwt.issuer", "taskmgr")

	v.SetDefault("ratelimit.rps", 1.67)
	v.SetDefault("ratelimit.burst", 100)

	v.SetDefault("metrics.path", "/metrics")

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")
	if err := v.ReadInConfig(); err != nil {

		var nf viper.ConfigFileNotFoundError
		if !errors.As(err, &nf) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	bindEnvs(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}

func bindEnvs(v *viper.Viper) {
	bindings := []struct{ key, env string }{
		{"app_env", "APP_ENV"},
		{"http.port", "HTTP_PORT"},
		{"log.level", "LOG_LEVEL"},
		{"mysql.dsn", "MYSQL_DSN"},
		{"mysql.max_open_conns", "MYSQL_MAX_OPEN_CONNS"},
		{"mysql.max_idle_conns", "MYSQL_MAX_IDLE_CONNS"},
		{"mysql.conn_max_lifetime", "MYSQL_CONN_MAX_LIFETIME"},
		{"mysql.conn_max_idle_time", "MYSQL_CONN_MAX_IDLE_TIME"},
		{"redis.addr", "REDIS_ADDR"},
		{"redis.password", "REDIS_PASSWORD"},
		{"redis.db", "REDIS_DB"},
		{"redis.pool_size", "REDIS_POOL_SIZE"},
		{"redis.dial_timeout", "REDIS_DIAL_TIMEOUT"},
		{"redis.read_timeout", "REDIS_READ_TIMEOUT"},
		{"redis.write_timeout", "REDIS_WRITE_TIMEOUT"},
		{"redis.tasks_cache_ttl", "REDIS_TASKS_CACHE_TTL"},
		{"jwt.secret", "JWT_SECRET"},
		{"jwt.ttl", "JWT_TTL"},
		{"jwt.issuer", "JWT_ISSUER"},
		{"ratelimit.rps", "RATE_LIMIT_RPS"},
		{"ratelimit.burst", "RATE_LIMIT_BURST"},
		{"metrics.path", "METRICS_PATH"},
	}
	for _, b := range bindings {
		_ = v.BindEnv(b.key, b.env)
	}
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.JWT.Secret) == "" {
		return errors.New("JWT_SECRET is required")
	}

	if c.AppEnv == "production" && len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 chars in production (got %d)", len(c.JWT.Secret))
	}
	if c.HTTP.Port == "" {
		return errors.New("HTTP port is empty")
	}
	if c.JWT.TTL <= 0 {
		return errors.New("JWT_TTL must be positive")
	}
	if c.RateLimit.RPS <= 0 {
		return errors.New("RATE_LIMIT_RPS must be positive")
	}
	if c.RateLimit.Burst <= 0 {
		return errors.New("RATE_LIMIT_BURST must be positive")
	}
	return nil
}
