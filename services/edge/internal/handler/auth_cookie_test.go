package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/config"
)

func TestRefreshCookieSetAndRead(t *testing.T) {
	conf := config.RefreshCookieConf{
		Name:          "refresh_token",
		Path:          "/v1/auth",
		SameSite:      "Lax",
		MaxAgeSeconds: 60,
	}
	rec := httptest.NewRecorder()

	setRefreshCookie(rec, "dev", conf, "token-1")

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "refresh_token" || cookie.Value != "token-1" {
		t.Fatalf("cookie = %+v, want refresh_token", cookie)
	}
	if !cookie.HttpOnly {
		t.Fatal("HttpOnly = false, want true")
	}
	if cookie.Secure {
		t.Fatal("Secure = true, want false for dev Lax cookie")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v, want Lax", cookie.SameSite)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	req.AddCookie(cookie)
	token, ok := refreshTokenFromCookie(req, conf)
	if !ok || token != "token-1" {
		t.Fatalf("refreshTokenFromCookie() = %q, %v, want token-1 true", token, ok)
	}
}

func TestRefreshCookieSecureDefaults(t *testing.T) {
	if !refreshCookieSecure("prod", config.RefreshCookieConf{SameSite: "Lax"}) {
		t.Fatal("refreshCookieSecure(prod) = false, want true")
	}
	if refreshCookieSecure("dev", config.RefreshCookieConf{SameSite: "Lax"}) {
		t.Fatal("refreshCookieSecure(dev) = true, want false")
	}
	if refreshCookieSecure("prod", config.RefreshCookieConf{AllowInsecureNonDev: true}) {
		t.Fatal("refreshCookieSecure(prod insecure override) = true, want false")
	}
	if !refreshCookieSecure("dev", config.RefreshCookieConf{SameSite: "None"}) {
		t.Fatal("refreshCookieSecure(SameSite=None) = false, want true")
	}
}

func TestClearRefreshCookie(t *testing.T) {
	conf := config.RefreshCookieConf{Name: "refresh_token", Path: "/v1/auth"}
	rec := httptest.NewRecorder()

	clearRefreshCookie(rec, "prod", conf)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.MaxAge != -1 || cookie.Value != "" {
		t.Fatalf("clear cookie = %+v, want MaxAge -1 and empty value", cookie)
	}
	if !cookie.HttpOnly || !cookie.Secure {
		t.Fatalf("clear cookie flags = HttpOnly:%v Secure:%v, want true true", cookie.HttpOnly, cookie.Secure)
	}
}
