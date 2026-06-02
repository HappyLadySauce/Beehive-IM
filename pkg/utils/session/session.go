package session

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

const sessionIDEntropyBytes = 32

// GenerateSessionID returns a high-entropy opaque session identifier.
// GenerateSessionID 返回高熵不透明会话标识。
func GenerateSessionID() (string, error) {
	raw := make([]byte, sessionIDEntropyBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("read session id entropy: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
