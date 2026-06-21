package cache

import (
	"context"
	"time"

	"github.com/example/go-project/internal/config"
	"github.com/redis/go-redis/v9"
)

// Cache определяет минимальный контракт для кеша. Позволяет подменять
// реализацию на мок в unit-тестах сервисного слоя.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	SetEX(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

type RedisCache struct {
	client *redis.Client
	ttl    time.Duration // дефолтный TTL для часто используемых ключей
}

func NewRedisCache(ctx context.Context, cfg config.RedisConfig) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Проверяем соединение — без этого первый запрос упадёт с задержкой.
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return &RedisCache{
		client: client,
		ttl:    cfg.TasksCacheTTL,
	}, nil
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (c *RedisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// SetEX — alias для Set; оставлен явно, чтобы в коде было понятно,
// что вызов подразумевает TTL-экспирацию.
func (c *RedisCache) SetEX(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.Set(ctx, key, value, ttl)
}

func (c *RedisCache) Del(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}
