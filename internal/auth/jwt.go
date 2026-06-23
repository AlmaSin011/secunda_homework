package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/example/go-project/internal/config"
)

const minSecretLen = 32

type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

func (c *Claims) UserID() uint64 {
	if c == nil || c.Subject == "" {
		return 0
	}
	id, err := strconv.ParseUint(c.Subject, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

var (
	ErrInvalidToken       = errors.New("invalid token")
	ErrExpiredToken       = errors.New("token expired")
	ErrTokenSignature     = errors.New("token signature mismatch")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewTokenManager(cfg config.JWTConfig) (*TokenManager, error) {
	if len(cfg.Secret) < minSecretLen {
		return nil, fmt.Errorf("jwt secret must be at least %d bytes (got %d)", minSecretLen, len(cfg.Secret))
	}
	if cfg.Issuer == "" {
		return nil, errors.New("jwt issuer is required")
	}
	if cfg.TTL <= 0 {
		return nil, errors.New("jwt ttl must be positive")
	}
	return &TokenManager{
		secret: []byte(cfg.Secret),
		issuer: cfg.Issuer,
		ttl:    cfg.TTL,
	}, nil
}

func (m *TokenManager) Issue(userID uint64, email string) (string, error) {
	now := time.Now()
	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(userID, 10),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(m.secret)
}

func (m *TokenManager) Parse(token string) (*Claims, error) {
	var claims Claims
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", ErrInvalidToken, t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithLeeway(30*time.Second))
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, ErrExpiredToken
		case errors.Is(err, jwt.ErrTokenSignatureInvalid):
			return nil, ErrTokenSignature
		default:
			return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
		}
	}
	if !parsed.Valid {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}

type PasswordHasher struct {
	cost int
}

func NewPasswordHasher(cost int) *PasswordHasher {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	return &PasswordHasher{cost: cost}
}

func (h *PasswordHasher) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(b), nil
}

func (h *PasswordHasher) Verify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
