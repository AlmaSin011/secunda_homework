package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/example/go-project/internal/auth"
	"github.com/example/go-project/internal/dto"
	"github.com/example/go-project/internal/entity"
)

// UserRepository — контракт хранения пользователей для AuthService.
type UserRepository interface {
	Create(ctx context.Context, u entity.User) (uint64, error)
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
}

var (
	ErrEmailTaken         = errors.New("email already taken")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type AuthService struct {
	repo   UserRepository
	tokens *auth.TokenManager
	pw     *auth.PasswordHasher
	now    func() time.Time
}

func NewAuthService(repo UserRepository, tokens *auth.TokenManager, pw *auth.PasswordHasher) *AuthService {
	return &AuthService{
		repo:   repo,
		tokens: tokens,
		pw:     pw,
		now:    time.Now,
	}
}

func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	existing, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("find by email: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	hash, err := s.pw.Hash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	now := s.now()
	u := entity.User{
		Email:        email,
		PasswordHash: hash,
		Name:         strings.TrimSpace(req.Name),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	id, err := s.repo.Create(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	u.ID = id

	tok, err := s.tokens.Issue(id, email)
	if err != nil {
		return nil, fmt.Errorf("issue token: %w", err)
	}
	return authResponse(&u, tok), nil
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	u, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("find by email: %w", err)
	}
	if u == nil {
		// Не сообщаем «юзера нет» — это утечка enumeration. Возвращаем
		// ErrInvalidCredentials, не отличимое от «пароль не сошёлся».
		return nil, ErrInvalidCredentials
	}
	if !s.pw.Verify(u.PasswordHash, req.Password) {
		return nil, ErrInvalidCredentials
	}

	tok, err := s.tokens.Issue(u.ID, u.Email)
	if err != nil {
		return nil, fmt.Errorf("issue token: %w", err)
	}
	return authResponse(u, tok), nil
}

func authResponse(u *entity.User, token string) *dto.AuthResponse {
	return &dto.AuthResponse{
		User: dto.AuthUser{
			ID:        u.ID,
			Email:     u.Email,
			Name:      u.Name,
			CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339),
		},
		Token: token,
	}
}
