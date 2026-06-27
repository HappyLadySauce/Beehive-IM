package wsproxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/security"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/ticket"
)

func TestProxyRejectsMissingTicket(t *testing.T) {
	proxy := NewProxy(Config{
		Tickets:       ticket.NewStore(30 * time.Second),
		OriginChecker: security.NewOriginChecker([]string{"https://app.example"}),
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "https://app.example")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertHTTPError(t, rec, "INVALID_WS_TICKET")
}

func TestProxyRejectsOriginMismatchBeforeAttach(t *testing.T) {
	store := ticket.NewStore(30 * time.Second)
	issued, err := store.Issue("user-1", "device-1", "session-1", "https://app.example")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	proxy := NewProxy(Config{
		Tickets:       store,
		OriginChecker: security.NewOriginChecker([]string{"https://app.example", "https://evil.example"}),
	})

	req := httptest.NewRequest(http.MethodGet, "/ws?ticket="+issued.Value, nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertHTTPError(t, rec, "INVALID_WS_TICKET")
}

func TestProxyRejectsExpiredTicketBeforeAttach(t *testing.T) {
	store := ticket.NewStore(time.Nanosecond)
	issued, err := store.Issue("user-1", "device-1", "session-1", "https://app.example")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	time.Sleep(time.Millisecond)
	proxy := NewProxy(Config{
		Tickets:       store,
		OriginChecker: security.NewOriginChecker([]string{"https://app.example"}),
	})

	req := httptest.NewRequest(http.MethodGet, "/ws?ticket="+issued.Value, nil)
	req.Header.Set("Origin", "https://app.example")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertHTTPError(t, rec, "INVALID_WS_TICKET")
}

func TestProxyRejectsUnknownOriginBeforeTicket(t *testing.T) {
	proxy := NewProxy(Config{
		Tickets:       ticket.NewStore(30 * time.Second),
		OriginChecker: security.NewOriginChecker([]string{"https://app.example"}),
	})

	req := httptest.NewRequest(http.MethodGet, "/ws?ticket=test", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	assertHTTPError(t, rec, "INVALID_ORIGIN")
}

func assertHTTPError(t *testing.T, rec *httptest.ResponseRecorder, wantCode string) {
	t.Helper()

	var body struct {
		Success   bool   `json:"success"`
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Success || body.ErrorCode != wantCode || body.Message == "" {
		t.Fatalf("body = %+v, want error_code %s", body, wantCode)
	}
}
