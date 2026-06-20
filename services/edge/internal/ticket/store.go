package ticket

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrMissingUserID  = errors.New("missing debug user id")
	ErrMissingTicket  = errors.New("missing ticket")
	ErrTicketNotFound = errors.New("ticket not found")
	ErrTicketExpired  = errors.New("ticket expired")
	ErrOriginMismatch = errors.New("ticket origin mismatch")
)

// Ticket contains the authenticated WebSocket handshake context.
// Ticket 保存已认证的 WebSocket 握手上下文。
type Ticket struct {
	Value     string
	UserID    string
	DeviceID  string
	SessionID string
	Origin    string
	ExpiresAt time.Time
}

// Store is a local one-time ticket store for the Edge MVP.
// Store 是 Edge MVP 阶段的本地一次性 ticket 存储。
type Store struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	entries map[string]Ticket
}

func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}

	return &Store{
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]Ticket),
	}
}

func (s *Store) TTL() time.Duration {
	return s.ttl
}

func (s *Store) Issue(userID, deviceID, sessionID, origin string) (Ticket, error) {
	if userID == "" {
		return Ticket{}, ErrMissingUserID
	}
	if deviceID == "" {
		deviceID = "web-" + mustRandomToken(8)
	}
	if sessionID == "" {
		sessionID = "sess-" + mustRandomToken(12)
	}

	value, err := randomToken(32)
	if err != nil {
		return Ticket{}, err
	}

	t := Ticket{
		Value:     value,
		UserID:    userID,
		DeviceID:  deviceID,
		SessionID: sessionID,
		Origin:    origin,
		ExpiresAt: s.now().UTC().Add(s.ttl),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(s.now().UTC())
	s.entries[value] = t
	return t, nil
}

func (s *Store) Consume(value, origin string) (Ticket, error) {
	if value == "" {
		return Ticket{}, ErrMissingTicket
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	t, ok := s.entries[value]
	if !ok {
		return Ticket{}, ErrTicketNotFound
	}
	delete(s.entries, value)

	if !now.Before(t.ExpiresAt) {
		return Ticket{}, ErrTicketExpired
	}
	if t.Origin != "" && t.Origin != origin {
		return Ticket{}, ErrOriginMismatch
	}

	return t, nil
}

func (s *Store) cleanupLocked(now time.Time) {
	for key, entry := range s.entries {
		if !now.Before(entry.ExpiresAt) {
			delete(s.entries, key)
		}
	}
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate ticket token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func mustRandomToken(size int) string {
	token, err := randomToken(size)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return token
}
