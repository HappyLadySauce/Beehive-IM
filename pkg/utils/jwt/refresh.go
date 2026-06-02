package jwt

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const refreshTokenEntropyBytes = 32

// GenerateRefreshToken returns a high-entropy opaque refresh token (URL-safe, no padding).
// GenerateRefreshToken 返回高熵不透明刷新令牌（URL 安全、无填充）。
func GenerateRefreshToken() (string, error) {
	raw := make([]byte, refreshTokenEntropyBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("read refresh token entropy: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// HashRefreshToken returns the SHA-256 hex digest used as the Redis lookup key.
// HashRefreshToken 返回用作 Redis 查找键的 SHA-256 十六进制摘要。
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
