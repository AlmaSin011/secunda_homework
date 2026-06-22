package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/example/go-project/internal/auth"
	"github.com/example/go-project/internal/config"
	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
)

type fakeUserRepo struct {
	mu    sync.Mutex
	byID  map[uint64]entity.User
	byKey map[string]uint64 // lower(email) → id
	next  uint64
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byID:  map[uint64]entity.User{},
		byKey: map[string]uint64{},
	}
}

func (f *fakeUserRepo) Create(_ context.Context, u entity.User) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.ToLower(u.Email)
	if _, exists := f.byKey[key]; exists {
		return 0, errors.New("duplicate") // страховка — service должен ловить раньше
	}
	f.next++
	u.ID = f.next
	f.byID[u.ID] = u
	f.byKey[key] = u.ID
	return u.ID, nil
}

func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (*entity.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byKey[strings.ToLower(email)]
	if !ok {
		return nil, nil
	}
	u := f.byID[id]
	return &u, nil
}

const (
	testSecret = "0123456789abcdef0123456789abcdef"
	testIssuer = "test-issuer"
)

func newTestAuthService(t *testing.T) (*AuthService, *fakeUserRepo) {
	t.Helper()
	repo := newFakeUserRepo()
	tm, err := auth.NewTokenManager(config.JWTConfig{
		Secret: testSecret,
		TTL:    time.Hour,
		Issuer: testIssuer,
	})
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	pw := auth.NewPasswordHasher(4) // bcrypt.MinCost — тесты быстрее
	return NewAuthService(repo, tm, pw), repo
}

func TestAuthService_Register_OK(t *testing.T) {
	svc, _ := newTestAuthService(t)
	resp, err := svc.Register(context.Background(), dto.RegisterRequest{
		Email:    "Alice@Example.com",
		Password: "supersecret",
		Name:     "  Alice  ",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if resp.User.ID == 0 {
		t.Error("User.ID is zero")
	}
	if resp.User.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com (lowercased)", resp.User.Email)
	}
	if resp.User.Name != "Alice" {
		t.Errorf("Name = %q, want %q (trimmed)", resp.User.Name, "Alice")
	}
	if resp.Token == "" {
		t.Error("Token is empty")
	}

	// Доп. проверка: токен парсится и sub=userID.
	tm, _ := auth.NewTokenManager(config.JWTConfig{
		Secret: testSecret, TTL: time.Hour, Issuer: testIssuer,
	})
	claims, err := tm.Parse(resp.Token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.UserID() != resp.User.ID {
		t.Errorf("token sub=%d, want %d", claims.UserID(), resp.User.ID)
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	svc, _ := newTestAuthService(t)
	ctx := context.Background()
	_, err := svc.Register(ctx, dto.RegisterRequest{
		Email: "a@b.c", Password: "supersecret", Name: "A",
	})
	if err != nil {
		t.Fatalf("first Register: %v", err)
	}
	_, err = svc.Register(ctx, dto.RegisterRequest{
		Email: "A@B.C", Password: "supersecret2", Name: "B",
	})
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("second Register err = %v, want ErrEmailTaken", err)
	}
}

func TestAuthService_Register_ValidationError(t *testing.T) {
	svc, _ := newTestAuthService(t)
	_, err := svc.Register(context.Background(), dto.RegisterRequest{
		Email: "bad", Password: "short", Name: "X",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAuthService_Register_HashStoredNotPlain(t *testing.T) {
	svc, repo := newTestAuthService(t)
	_, err := svc.Register(context.Background(), dto.RegisterRequest{
		Email: "a@b.c", Password: "supersecret", Name: "A",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	u, err := repo.FindByEmail(context.Background(), "a@b.c")
	if err != nil || u == nil {
		t.Fatalf("FindByEmail: %v %v", err, u)
	}
	if u.PasswordHash == "supersecret" {
		t.Error("PasswordHash equals plaintext — bcrypt was not applied")
	}
	if !strings.HasPrefix(u.PasswordHash, "$2") {
		t.Errorf("PasswordHash %q does not look like bcrypt", u.PasswordHash)
	}
}

func TestAuthService_Login_OK(t *testing.T) {
	svc, _ := newTestAuthService(t)
	ctx := context.Background()
	_, err := svc.Register(ctx, dto.RegisterRequest{
		Email: "a@b.c", Password: "supersecret", Name: "A",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	resp, err := svc.Login(ctx, dto.LoginRequest{
		Email: "a@b.c", Password: "supersecret",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if resp.User.Email != "a@b.c" {
		t.Errorf("Email = %q, want a@b.c", resp.User.Email)
	}
	if resp.Token == "" {
		t.Error("Token is empty")
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	svc, _ := newTestAuthService(t)
	ctx := context.Background()
	_, _ = svc.Register(ctx, dto.RegisterRequest{
		Email: "a@b.c", Password: "supersecret", Name: "A",
	})
	_, err := svc.Login(ctx, dto.LoginRequest{
		Email: "a@b.c", Password: "wrong-pwd",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_UnknownEmail(t *testing.T) {
	svc, _ := newTestAuthService(t)
	_, err := svc.Login(context.Background(), dto.LoginRequest{
		Email: "nobody@nowhere.io", Password: "whatever",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials (no enumeration leak)", err)
	}
}

func TestAuthService_Login_ValidationError(t *testing.T) {
	svc, _ := newTestAuthService(t)
	_, err := svc.Login(context.Background(), dto.LoginRequest{
		Email: "", Password: "",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAuthService_Register_PreservesCreatedAt(t *testing.T) {
	svc, repo := newTestAuthService(t)
	resp, err := svc.Register(context.Background(), dto.RegisterRequest{
		Email: "a@b.c", Password: "supersecret", Name: "A",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if resp.User.CreatedAt == "" {
		t.Error("CreatedAt is empty")
	}
	u, _ := repo.FindByEmail(context.Background(), "a@b.c")
	if u.CreatedAt.IsZero() {
		t.Error("repo CreatedAt is zero — time injection missing?")
	}
}
