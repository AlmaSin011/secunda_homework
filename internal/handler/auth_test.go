package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/example/go-project/internal/auth"
	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
	"github.com/example/go-project/internal/handler"
	"github.com/example/go-project/internal/middleware"
	"github.com/example/go-project/internal/service"
	"github.com/example/go-project/internal/utills"
)

type fakeUserRepo struct {
	byEmail map[string]*entity.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byEmail: map[string]*entity.User{}}
}

func (r *fakeUserRepo) Create(_ context.Context, u entity.User) (uint64, error) {
	if _, ok := r.byEmail[u.Email]; ok {
		return 0, service.ErrEmailTaken
	}
	u.ID = uint64(len(r.byEmail) + 1)
	r.byEmail[u.Email] = &u
	return u.ID, nil
}

func (r *fakeUserRepo) FindByEmail(_ context.Context, email string) (*entity.User, error) {
	return r.byEmail[email], nil
}

func mustTokenManager(t *testing.T) *auth.TokenManager {
	t.Helper()
	tm, err := auth.NewTokenManager(utills.JWTConfigForTest())
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	return tm
}

func newTestRouter(authSvc *service.AuthService, tm *auth.TokenManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	authH := handler.NewAuthHandler(authSvc, nil)

	r.POST("/api/v1/register", authH.Register)
	r.POST("/api/v1/login", authH.Login)
	r.GET("/api/v1/protected",
		middleware.RequireAuth(tm, nil),
		func(c *gin.Context) {
			uid, _ := middleware.UserIDFromContext(c)
			c.JSON(http.StatusOK, gin.H{"uid": uid})
		},
	)
	return r
}

func newService(t *testing.T) (*service.AuthService, *fakeUserRepo) {
	t.Helper()
	tm := mustTokenManager(t)
	repo := newFakeUserRepo()
	hasher := auth.NewPasswordHasher(bcrypt.MinCost)
	svc := service.NewAuthService(repo, tm, hasher)
	return svc, repo
}

func TestRegister_Success(t *testing.T) {
	svc, _ := newService(t)
	r := newTestRouter(svc, mustTokenManager(t))

	body, _ := json.Marshal(dto.RegisterRequest{
		Email:    "John@example.com",
		Password: "password123",
		Name:     "John",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"token":`) {
		t.Fatalf("expected token in body, got %s", w.Body.String())
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc, repo := newService(t)
	repo.byEmail["x@example.com"] = &entity.User{Email: "x@example.com"}
	r := newTestRouter(svc, mustTokenManager(t))

	body, _ := json.Marshal(dto.RegisterRequest{
		Email:    "x@example.com",
		Password: "password123",
		Name:     "X",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"email already taken"`) {
		t.Fatalf("want conflict message, got %s", w.Body.String())
	}
}

func TestRegister_ValidationError(t *testing.T) {
	svc, _ := newService(t)
	r := newTestRouter(svc, mustTokenManager(t))

	// пустой email
	body, _ := json.Marshal(dto.RegisterRequest{Email: "", Password: "password123", Name: "X"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"code":"validation_error"`) {
		t.Fatalf("want validation_error code, got %s", w.Body.String())
	}
}

func TestLogin_OK(t *testing.T) {
	svc, repo := newService(t)
	// кладём пользователя в обход Register, чтобы не зависеть от bcrypt
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	repo.byEmail["test@example.com"] = &entity.User{
		ID:           1,
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Name:         "Test",
	}
	r := newTestRouter(svc, mustTokenManager(t))

	body, _ := json.Marshal(dto.LoginRequest{Email: "bob@example.com", Password: "password123"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"token":`) {
		t.Fatalf("expected token, got %s", w.Body.String())
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, repo := newService(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("goodpass"), bcrypt.MinCost)
	repo.byEmail["u@example.com"] = &entity.User{
		ID:           1,
		Email:        "u@example.com",
		PasswordHash: string(hash),
		Name:         "U",
	}
	r := newTestRouter(svc, mustTokenManager(t))

	body, _ := json.Marshal(dto.LoginRequest{Email: "u@example.com", Password: "WRONG"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestLogin_UnknownUser_NotDistinguishable(t *testing.T) {

	svc, _ := newService(t)
	r := newTestRouter(svc, mustTokenManager(t))

	body, _ := json.Marshal(dto.LoginRequest{Email: "nobody@example.com", Password: "any"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
	// важно: в ответе НЕТ слова "not found" — оно бы лило enumeration.
	if strings.Contains(w.Body.String(), "not found") {
		t.Fatalf("enumeration leak: %s", w.Body.String())
	}
}

func TestRequireAuth_NoHeader(t *testing.T) {
	r := newTestRouter(nil, mustTokenManager(t))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	r := newTestRouter(nil, mustTokenManager(t))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}
