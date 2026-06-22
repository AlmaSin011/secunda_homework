package middleware

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/example/go-project/internal/auth"
	"github.com/example/go-project/internal/config"
)

const (
	testSecret = "0123456789abcdef0123456789abcdef"
	testIssuer = "test-issuer"
)

func newTestTM(t *testing.T) *auth.TokenManager {
	t.Helper()
	tm, err := auth.NewTokenManager(config.JWTConfig{
		Secret: testSecret,
		TTL:    time.Hour,
		Issuer: testIssuer,
	})
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	return tm
}

func newRouter(tm *auth.TokenManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireAuth(tm, slog.New(slog.NewTextHandler(io.Discard, nil))))

	r.GET("/whoami", func(c *gin.Context) {
		uid, _ := UserIDFromContext(c)
		email, _ := EmailFromContext(c)
		c.JSON(http.StatusOK, gin.H{"user_id": uid, "email": email})
	})
	return r
}

func signToken(t *testing.T, tm *auth.TokenManager, userID uint64, email string, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uintToString(userID),
			Issuer:    testIssuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	})
	raw, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return raw
}

func uintToString(u uint64) string {
	const digits = "0123456789"
	if u == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for u > 0 {
		i--
		b[i] = digits[u%10]
		u /= 10
	}
	return string(b[i:])
}

func signTokenWithSub(t *testing.T, subject, issuer string, exp time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":   subject,
		"iss":   issuer,
		"exp":   exp.Unix(),
		"iat":   time.Now().Unix(),
		"email": "x@y.z",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	raw, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return raw
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	r := newRouter(newTestTM(t))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	var body map[string]map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body json: %v", err)
	}
	if body["error"]["code"] != "unauthorized" {
		t.Errorf("code = %q, want unauthorized", body["error"]["code"])
	}
}

func TestRequireAuth_BadPrefix(t *testing.T) {
	r := newRouter(newTestTM(t))

	for _, h := range []string{"Basic abc", "Token xyz", "Bearer", "Bearer "} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
		req.Header.Set("Authorization", h)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("header %q: status = %d, want 401", h, w.Code)
		}
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	r := newRouter(newTestTM(t))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	tm := newTestTM(t)
	r := newRouter(tm)

	raw := signToken(t, tm, 1, "x@y.z", time.Now().Add(-time.Hour))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestRequireAuth_ValidToken(t *testing.T) {
	tm := newTestTM(t)
	r := newRouter(tm)

	raw := signToken(t, tm, 42, "alice@example.com", time.Now().Add(time.Hour))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Logf("body: %s", w.Body.String())
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body struct {
		UserID uint64 `json:"user_id"`
		Email  string `json:"email"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if body.UserID != 42 {
		t.Errorf("user_id = %d, want 42", body.UserID)
	}
	if body.Email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", body.Email)
	}
}

func TestRequireAuth_BadSubject(t *testing.T) {
	tm := newTestTM(t)
	r := newRouter(tm)

	raw := signTokenWithSub(t, "not-a-number", testIssuer, time.Now().Add(time.Hour))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestRequireAuth_BadSignature(t *testing.T) {
	tm := newTestTM(t)
	r := newRouter(tm)

	other := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "1",
		"iss":   testIssuer,
		"exp":   time.Now().Add(time.Hour).Unix(),
		"email": "x@y.z",
	})
	raw, err := other.SignedString([]byte("ffffffffffffffffffffffffffffffff"))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestUserIDFromContext_Missing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if _, ok := UserIDFromContext(c); ok {
		t.Error("UserIDFromContext should return false without middleware")
	}
	if _, ok := EmailFromContext(c); ok {
		t.Error("EmailFromContext should return false without middleware")
	}
}

func TestExtractBearer_Cases(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"abc", ""},
		{"Bearer", ""},
		{"Bearer ", ""},
		{"Bearer abc", "abc"},
		{"bearer abc", "abc"}, // регистронезависимо
		{"BEARER   abc  ", "abc"},
		{"Basic abc", ""},
	}
	for _, tc := range cases {
		if got := extractBearer(tc.in); got != tc.want {
			t.Errorf("extractBearer(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
