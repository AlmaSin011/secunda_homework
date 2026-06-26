package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/example/go-project/internal/cache"
	"github.com/example/go-project/internal/dto"
)

// TTL по умолчанию для кеша списка задач.
const taskListCacheTTL = 5 * time.Minute

func taskListCacheKey(f dto.TaskFilter) string {
	status := string(f.Status)
	if status == "" {
		status = "any"
	}
	return fmt.Sprintf("task:list:team:%d:status:%s:page:%d", f.TeamID, status, f.Page)
}

type taskListCache struct {
	cache cache.Cache
	ttl   time.Duration
}

func newTaskListCache(c cache.Cache) *taskListCache {
	if c == nil {
		return nil // допустимо: TaskService умеет работать без кеша
	}
	return &taskListCache{cache: c, ttl: taskListCacheTTL}
}

var ErrCacheMiss = errors.New("cache miss")

func (c *taskListCache) Get(ctx context.Context, f dto.TaskFilter) (*dto.TasksListResponse, error) {
	if c == nil {
		return nil, ErrCacheMiss
	}
	key := taskListCacheKey(f)
	raw, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, ErrCacheMiss // любой сбой кеша = промах (cache-aside)
	}
	if raw == "" {
		return nil, ErrCacheMiss
	}
	var out dto.TasksListResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, ErrCacheMiss // битый кеш = промах
	}
	return &out, nil
}

func (c *taskListCache) Store(ctx context.Context, f dto.TaskFilter, resp *dto.TasksListResponse) error {
	if c == nil || resp == nil {
		return nil
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return nil // не критично: БД-источник всегда прав
	}
	return c.cache.Set(ctx, taskListCacheKey(f), string(raw), c.ttl)
}

func (c *taskListCache) Invalidate(ctx context.Context, teamID uint64) error {
	if c == nil {
		return nil
	}
	statuses := []string{"any", "todo", "in_progress", "done"}
	for _, s := range statuses {
		// лимит ставим заведомо больше любой разумной пагинации.
		for page := 1; page <= 100; page++ {
			key := fmt.Sprintf("task:list:team:%d:status:%s:page:%d", teamID, s, page)
			if err := c.cache.Del(ctx, key); err != nil {
				return err
			}
		}
	}
	return nil
}
