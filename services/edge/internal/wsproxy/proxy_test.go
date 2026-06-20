package wsproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/ticket"
)

func TestProxyRejectsMissingTicket(t *testing.T) {
	proxy := NewProxy(Config{
		Tickets: ticket.NewStore(30 * time.Second),
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestProxyRejectsOriginMismatchBeforeAttach(t *testing.T) {
	store := ticket.NewStore(30 * time.Second)
	issued, err := store.Issue("user-1", "device-1", "session-1", "https://app.example")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	proxy := NewProxy(Config{
		Tickets: store,
	})

	req := httptest.NewRequest(http.MethodGet, "/ws?ticket="+issued.Value, nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestProxyRejectsExpiredTicketBeforeAttach(t *testing.T) {
	store := ticket.NewStore(time.Nanosecond)
	issued, err := store.Issue("user-1", "device-1", "session-1", "")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	time.Sleep(time.Millisecond)
	proxy := NewProxy(Config{
		Tickets: store,
	})

	req := httptest.NewRequest(http.MethodGet, "/ws?ticket="+issued.Value, nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
