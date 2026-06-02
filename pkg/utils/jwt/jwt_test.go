package jwt

import (
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

func TestTokenIssuerMustMatchExpectedIssuer(t *testing.T) {
	token, err := GenerateToken(
		"1",
		"alice",
		"version-1",
		"Beehive-IM",
		"12345678901234567890123456789012",
		jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
	)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if _, err := ParseToken(token, "12345678901234567890123456789012", "other-issuer"); err == nil {
		t.Fatal("ParseToken() error = nil, want issuer mismatch error")
	}

	claims, err := ParseToken(token, "12345678901234567890123456789012", "Beehive-IM")
	if err != nil {
		t.Fatalf("ParseToken() with matching issuer error = %v", err)
	}
	if claims.Issuer != "Beehive-IM" {
		t.Fatalf("claims.Issuer = %q, want %q", claims.Issuer, "Beehive-IM")
	}
}
