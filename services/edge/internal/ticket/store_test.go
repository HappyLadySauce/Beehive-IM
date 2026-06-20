package ticket

import (
	"errors"
	"testing"
	"time"
)

func TestStoreIssueAndConsumeOnce(t *testing.T) {
	base := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	store := NewStore(30 * time.Second)
	store.now = func() time.Time {
		return base
	}

	issued, err := store.Issue("user-1", "device-1", "session-1", "https://app.example")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	consumed, err := store.Consume(issued.Value, "https://app.example")
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if consumed.UserID != "user-1" || consumed.DeviceID != "device-1" || consumed.SessionID != "session-1" {
		t.Fatalf("Consume() returned unexpected ticket: %+v", consumed)
	}

	_, err = store.Consume(issued.Value, "https://app.example")
	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("Consume() second error = %v, want %v", err, ErrTicketNotFound)
	}
}

func TestStoreConsumeExpiredTicket(t *testing.T) {
	base := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	now := base
	store := NewStore(30 * time.Second)
	store.now = func() time.Time {
		return now
	}

	issued, err := store.Issue("user-1", "", "", "")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	now = base.Add(31 * time.Second)
	_, err = store.Consume(issued.Value, "")
	if !errors.Is(err, ErrTicketExpired) {
		t.Fatalf("Consume() error = %v, want %v", err, ErrTicketExpired)
	}
}

func TestStoreConsumeRejectsOriginMismatch(t *testing.T) {
	store := NewStore(30 * time.Second)
	issued, err := store.Issue("user-1", "", "", "https://app.example")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	_, err = store.Consume(issued.Value, "https://evil.example")
	if !errors.Is(err, ErrOriginMismatch) {
		t.Fatalf("Consume() error = %v, want %v", err, ErrOriginMismatch)
	}
}

func TestStoreConsumeRejectsMissingBoundOrigin(t *testing.T) {
	store := NewStore(30 * time.Second)
	issued, err := store.Issue("user-1", "", "", "https://app.example")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	_, err = store.Consume(issued.Value, "")
	if !errors.Is(err, ErrOriginMismatch) {
		t.Fatalf("Consume() error = %v, want %v", err, ErrOriginMismatch)
	}
}

func TestStoreIssueRequiresUserID(t *testing.T) {
	store := NewStore(30 * time.Second)
	_, err := store.Issue("", "", "", "")
	if !errors.Is(err, ErrMissingUserID) {
		t.Fatalf("Issue() error = %v, want %v", err, ErrMissingUserID)
	}
}
