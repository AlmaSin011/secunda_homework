// Package testkit — общие хелперы для unit-тестов.
package testkit

import (
	"time"

	"github.com/example/go-project/internal/config"
)

// JWTConfigForTest — единая фабрика тестового JWT-конфига (secret ≥ 32 байт).
func JWTConfigForTest() config.JWTConfig {
	return config.JWTConfig{
		Secret: "0123456789abcdef0123456789abcdef", // 32 bytes
		Issuer: "test-issuer",
		TTL:    time.Hour,
	}
}
