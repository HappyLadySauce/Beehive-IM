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
		"session-1",
		"device-1",
		"web",
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

func TestTokenContainsDeviceSessionClaims(t *testing.T) {
	token, err := GenerateToken(
		"1",
		"alice",
		"session-1",
		"device-1",
		"web",
		"version-1",
		"Beehive-IM",
		"12345678901234567890123456789012",
		jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
	)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	claims, err := ParseToken(token, "12345678901234567890123456789012", "Beehive-IM")
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.SessionID != "session-1" || claims.DeviceID != "device-1" || claims.Platform != "web" || claims.Version != "version-1" {
		t.Fatalf("claims session/device/platform/version = %q/%q/%q/%q, want session-1/device-1/web/version-1", claims.SessionID, claims.DeviceID, claims.Platform, claims.Version)
	}
}

func TestParseTokenRejectsNonHS256Algorithm(t *testing.T) {
	claims := Claims{
		UserID:    "1",
		Username:  "alice",
		SessionID: "session-1",
		DeviceID:  "device-1",
		Platform:  "web",
		Version:   "version-1",
		RegisteredClaims: jwtv5.RegisteredClaims{
			ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
			Issuer:    "Beehive-IM",
			Subject:   "1",
		},
	}
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS384, claims).SignedString([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	if _, err := ParseToken(token, "12345678901234567890123456789012", "Beehive-IM"); err == nil {
		t.Fatal("ParseToken() error = nil, want algorithm mismatch error")
	}
}

func TestParseTokenRejectsExpiredToken(t *testing.T) {
	token, err := GenerateToken(
		"1",
		"alice",
		"session-1",
		"device-1",
		"web",
		"version-1",
		"Beehive-IM",
		"12345678901234567890123456789012",
		jwtv5.NewNumericDate(time.Now().Add(-time.Hour)),
	)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if _, err := ParseToken(token, "12345678901234567890123456789012", "Beehive-IM"); err == nil {
		t.Fatal("ParseToken() error = nil, want expired token error")
	}
}
