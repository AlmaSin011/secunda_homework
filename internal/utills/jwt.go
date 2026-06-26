package utills

import (
	"time"

	"github.com/example/go-project/internal/config"
)

func JWTConfigForTest() config.JWTConfig {
	return config.JWTConfig{
		Secret: "0123456789abcdef0123456789abcdef", // 32 bytes
		Issuer: "test-issuer",
		TTL:    time.Hour,
	}
}
