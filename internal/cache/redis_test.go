package cache

import (
	"context"
	"testing"
	"time"
)

// FakeCache — простая in-memory реализация Cache для проверки контракта
type FakeCache struct {
	store map[string]string
	ttl   map[string]time.Time
}

func NewFake() *FakeCache {
	return &FakeCache{store: map[string]string{}, ttl: map[string]time.Time{}}
}

func (f *FakeCache) Get(_ context.Context, key string) (string, error) {
	if exp, ok := f.ttl[key]; ok && time.Now().After(exp) {
		delete(f.store, key)
		delete(f.ttl, key)
		return "", nil
	}
	return f.store[key], nil
}

func (f *FakeCache) Set(_ context.Context, key, value string, ttl time.Duration) error {
	f.store[key] = value
	f.ttl[key] = time.Now().Add(ttl)
	return nil
}

func (f *FakeCache) SetEX(ctx context.Context, key, value string, ttl time.Duration) error {
	return f.Set(ctx, key, value, ttl)
}

func (f *FakeCache) Del(_ context.Context, key string) error {
	delete(f.store, key)
	delete(f.ttl, key)
	return nil
}

var _ Cache = (*FakeCache)(nil)

func TestFakeCacheRoundTrip(t *testing.T) {
	f := NewFake()
	ctx := context.Background()

	if err := f.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := f.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "v" {
		t.Errorf("Get = %q, want %q", got, "v")
	}
	if err := f.Del(ctx, "k"); err != nil {
		t.Fatalf("Del: %v", err)
	}
	got, _ = f.Get(ctx, "k")
	if got != "" {
		t.Errorf("after Del Get = %q, want empty", got)
	}
}

func TestFakeCacheExpiry(t *testing.T) {
	f := NewFake()
	ctx := context.Background()

	_ = f.Set(ctx, "k", "v", 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)

	got, _ := f.Get(ctx, "k")
	if got != "" {
		t.Errorf("expired key returned %q, want empty", got)
	}
}

func TestFakeCacheSetEX(t *testing.T) {
	f := NewFake()
	ctx := context.Background()
	if err := f.SetEX(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("SetEX: %v", err)
	}
	got, _ := f.Get(ctx, "k")
	if got != "v" {
		t.Errorf("Get after SetEX = %q, want %q", got, "v")
	}
}
