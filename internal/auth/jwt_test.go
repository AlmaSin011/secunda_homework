package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/example/go-project/internal/config"
)

// for test
const secret32 = "0123456789abcdef0123456789abcdef"

func newTestTM(t *testing.T) *TokenManager {
	t.Helper()
	tm, err := NewTokenManager(config.JWTConfig{
		Secret: secret32,
		TTL:    time.Hour,
		Issuer: "test-issuer",
	})
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	return tm
}

func TestNewTokenManager_ShortSecret(t *testing.T) {
	_, err := NewTokenManager(config.JWTConfig{
		Secret: "short",
		TTL:    time.Hour,
		Issuer: "x",
	})
	if err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestNewTokenManager_MissingIssuer(t *testing.T) {
	_, err := NewTokenManager(config.JWTConfig{
		Secret: secret32,
		TTL:    time.Hour,
		Issuer: "",
	})
	if err == nil {
		t.Fatal("expected error for empty issuer")
	}
}

func TestNewTokenManager_NonPositiveTTL(t *testing.T) {
	_, err := NewTokenManager(config.JWTConfig{
		Secret: secret32,
		TTL:    0,
		Issuer: "x",
	})
	if err == nil {
		t.Fatal("expected error for non-positive TTL")
	}
}

func TestTokenManager_IssueParse_RoundTrip(t *testing.T) {
	tm := newTestTM(t)

	tok, err := tm.Issue(42, "alice@example.com")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if tok == "" {
		t.Fatal("token is empty")
	}

	claims, err := tm.Parse(tok)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := claims.UserID(); got != 42 {
		t.Errorf("UserID = %d, want 42", got)
	}
	if claims.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", claims.Email)
	}
	if claims.Issuer != "test-issuer" {
		t.Errorf("Issuer = %q, want test-issuer", claims.Issuer)
	}
	if claims.ExpiresAt == nil {
		t.Error("ExpiresAt is nil")
	}
}

func TestTokenManager_Parse_Expired(t *testing.T) {
	tm := newTestTM(t)

	claims := Claims{
		Email: "x@y.z",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			Issuer:    "test-issuer",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	raw, err := tok.SignedString(tm.secret)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}

	_, err = tm.Parse(raw)
	if !errors.Is(err, ErrExpiredToken) {
		t.Errorf("Parse expired token err = %v, want ErrExpiredToken", err)
	}
}

func TestTokenManager_Parse_BadSignature(t *testing.T) {
	tm := newTestTM(t)
	tok, err := tm.Issue(1, "x@y.z")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Подменяем менеджер на другой секрет — подпись не сойдётся.
	other, err := NewTokenManager(config.JWTConfig{
		Secret: "ffffffffffffffffffffffffffffffff",
		TTL:    time.Hour,
		Issuer: "test-issuer",
	})
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	_, err = other.Parse(tok)
	if !errors.Is(err, ErrTokenSignature) {
		t.Errorf("Parse with other secret err = %v, want ErrTokenSignature", err)
	}
}

func TestTokenManager_Parse_BadIssuer(t *testing.T) {
	tm := newTestTM(t)
	other, err := NewTokenManager(config.JWTConfig{
		Secret: secret32,
		TTL:    time.Hour,
		Issuer: "other-issuer",
	})
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}

	// Токен подписан tm, но проверяется менеджером с другим issuer.
	tok, err := tm.Issue(1, "x@y.z")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	_, err = other.Parse(tok)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Parse with other issuer err = %v, want ErrInvalidToken", err)
	}
}

func TestTokenManager_Parse_Malformed(t *testing.T) {
	tm := newTestTM(t)
	for _, garbage := range []string{"", "not-a-jwt", "a.b.c", "....."} {
		_, err := tm.Parse(garbage)
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("Parse(%q) err = %v, want ErrInvalidToken", garbage, err)
		}
	}
}

func TestClaims_UserID_BadSubject(t *testing.T) {
	c := &Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "not-a-number"}}
	if got := c.UserID(); got != 0 {
		t.Errorf("UserID for bad sub = %d, want 0", got)
	}
	c = &Claims{}
	if got := c.UserID(); got != 0 {
		t.Errorf("UserID for empty sub = %d, want 0", got)
	}
	c = nil
	if got := c.UserID(); got != 0 {
		t.Errorf("UserID for nil claims = %d, want 0", got)
	}
}

func TestPasswordHasher_RoundTrip(t *testing.T) {
	pw := NewPasswordHasher(4) // bcrypt.MinCost — тесты быстрее
	hash, err := pw.Hash("secret-password")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !pw.Verify(hash, "secret-password") {
		t.Error("Verify of correct password failed")
	}
	if pw.Verify(hash, "wrong") {
		t.Error("Verify of wrong password returned true")
	}
}

func TestPasswordHasher_UniqueSalts(t *testing.T) {
	pw := NewPasswordHasher(4)
	h1, _ := pw.Hash("same")
	h2, _ := pw.Hash("same")
	if h1 == h2 {
		t.Error("two hashes of the same password are equal — bcrypt salt is missing")
	}
}

func TestPasswordHasher_Verify_InvalidHash(t *testing.T) {
	pw := NewPasswordHasher(4)
	if pw.Verify("not-a-bcrypt-hash", "anything") {
		t.Error("Verify of malformed hash returned true")
	}
}

func TestNewPasswordHasher_NormalizesCost(t *testing.T) {

	pw := NewPasswordHasher(0)
	if pw.cost != 0 {

		if pw.cost < 4 || pw.cost > 31 {
			t.Errorf("cost=%d outside bcrypt range", pw.cost)
		}
	}
	pw = NewPasswordHasher(100)
	if pw.cost < 4 || pw.cost > 31 {
		t.Errorf("cost=%d outside bcrypt range after normalization", pw.cost)
	}
	pw = NewPasswordHasher(6) // валидное значение — должно сохраниться
	if pw.cost != 6 {
		t.Errorf("cost=%d, want 6", pw.cost)
	}
}
