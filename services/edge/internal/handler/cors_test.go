package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/config"
)

func TestCORSMiddlewareAllowsConfiguredPreflight(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	handler := NewCORSMiddleware(config.SecurityConf{
		AllowedOrigins: []string{"http://localhost:5173"},
	})(next)

	req := httptest.NewRequest(http.MethodOptions, "/v1/messages/sync", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, X-Device-Id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("next handler was called for preflight")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allow origin = %q, want configured origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("allow credentials = %q, want true", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != corsAllowedHeaders {
		t.Fatalf("allow headers = %q, want %q", got, corsAllowedHeaders)
	}
}

func TestCORSMiddlewareRejectsUnknownOriginWithJSON(t *testing.T) {
	handler := NewCORSMiddleware(config.SecurityConf{
		AllowedOrigins: []string{"http://localhost:5173"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/v1/messages/sync", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	var body errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Success || body.ErrorCode != "INVALID_ORIGIN" {
		t.Fatalf("body = %+v, want INVALID_ORIGIN failure", body)
	}
}

func TestCORSMiddlewarePassesRequestsWithoutOrigin(t *testing.T) {
	called := false
	handler := NewCORSMiddleware(config.SecurityConf{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler was not called")
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}
