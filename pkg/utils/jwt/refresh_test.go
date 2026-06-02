package jwt

import (
	"testing"
)

func TestGenerateRefreshTokenIsUniqueAndHashable(t *testing.T) {
	a, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}
	b, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() second error = %v", err)
	}
	if a == b {
		t.Fatal("expected distinct refresh tokens")
	}
	if len(HashRefreshToken(a)) != 64 {
		t.Fatalf("HashRefreshToken() len = %d, want 64 hex chars", len(HashRefreshToken(a)))
	}
	if HashRefreshToken(a) != HashRefreshToken(a) {
		t.Fatal("hash should be deterministic for the same token")
	}
	if HashRefreshToken(a) == HashRefreshToken(b) {
		t.Fatal("different tokens should not share the same hash")
	}
}
