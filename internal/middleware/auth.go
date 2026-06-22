package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/example/go-project/internal/auth"
)

const (
	CtxUserID = "userID"
	CtxEmail  = "userEmail"
)

func RequireAuth(tm *auth.TokenManager, log *slog.Logger) gin.HandlerFunc {
	if log == nil {
		log = slog.Default()
	}
	return func(c *gin.Context) {
		raw := extractBearer(c.GetHeader("Authorization"))
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "unauthorized", "message": "missing bearer token"},
			})
			return
		}

		claims, err := tm.Parse(raw)
		if err != nil {
			log.Warn("auth parse failed",
				slog.String("err", err.Error()),
				slog.String("client_ip", c.ClientIP()),
			)

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "unauthorized", "message": "invalid token"},
			})
			return
		}

		userID := claims.UserID()
		if userID == 0 {
			log.Warn("auth: bad subject claim",
				slog.String("subject", claims.Subject),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "unauthorized", "message": "invalid token"},
			})
			return
		}

		c.Set(CtxUserID, userID)
		c.Set(CtxEmail, claims.Email)
		c.Next()
	}
}

func extractBearer(header string) string {
	if header == "" {
		return ""
	}
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

func UserIDFromContext(c *gin.Context) (uint64, bool) {
	v, ok := c.Get(CtxUserID)
	if !ok {
		return 0, false
	}
	id, ok := v.(uint64)
	return id, ok
}

func EmailFromContext(c *gin.Context) (string, bool) {
	v, ok := c.Get(CtxEmail)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
