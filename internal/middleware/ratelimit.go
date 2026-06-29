package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/example/go-project/internal/dto"
)

// PerUserLimiter — token bucket на каждого авторизованного пользователя.
// Ключ — userID из JWT. Неавторизованные запросы идут под общим лимитом,
// чтобы перебор токенов снаружи не обходил throttling.
type PerUserLimiter struct {
	rps   rate.Limit
	burst int

	mu       sync.Mutex
	perUser  map[uint64]*entry
	fallback *entry
}

type entry struct {
	limiter *rate.Limiter
	lastUse time.Time
}

func NewPerUserLimiter(rps float64, burst int) *PerUserLimiter {
	return &PerUserLimiter{
		rps:      rate.Limit(rps),
		burst:    burst,
		perUser:  make(map[uint64]*entry),
		fallback: &entry{limiter: rate.NewLimiter(rate.Limit(rps), burst)},
	}
}

func (l *PerUserLimiter) get(userID uint64) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	if userID == 0 {
		return l.fallback.limiter
	}
	e, ok := l.perUser[userID]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.perUser[userID] = e
	}
	e.lastUse = time.Now()
	return e.limiter
}

// Run обходит map и удаляет записи, не использованные дольше ttl.
func (l *PerUserLimiter) gc(ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-ttl)
	for k, e := range l.perUser {
		if e.lastUse.Before(cutoff) {
			delete(l.perUser, k)
		}
	}
}

func RateLimit(l *PerUserLimiter) gin.HandlerFunc {
	if l == nil {

		return func(c *gin.Context) { c.Next() }
	}
	go func() {
		// раз в час чистим неактивных.
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for range t.C {
			l.gc(time.Hour)
		}
	}()
	return func(c *gin.Context) {
		uid, _ := UserIDFromContext(c)
		if !l.get(uid).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, dto.NewError(
				dto.CodeRateLimited, "too many requests",
			))
			return
		}
		c.Next()
	}
}
