package wsproxy

import (
	"context"
	"testing"
)

func TestHubDeliverByConnAndSession(t *testing.T) {
	hub := NewHub()
	outbound := make(chan []byte, 2)
	hub.Register("conn-1", "session-1", outbound)
	defer hub.Unregister("conn-1", "session-1")

	if !hub.Deliver(context.Background(), PushTarget{ConnID: "conn-1"}, []byte("by-conn")) {
		t.Fatal("Deliver() by conn = false, want true")
	}
	if !hub.Deliver(context.Background(), PushTarget{SessionID: "session-1"}, []byte("by-session")) {
		t.Fatal("Deliver() by session = false, want true")
	}
	if got := string(<-outbound); got != "by-conn" {
		t.Fatalf("first payload = %q, want by-conn", got)
	}
	if got := string(<-outbound); got != "by-session" {
		t.Fatalf("second payload = %q, want by-session", got)
	}
}

func TestHubDeliverUnknownTarget(t *testing.T) {
	hub := NewHub()
	if hub.Deliver(context.Background(), PushTarget{ConnID: "missing"}, []byte("payload")) {
		t.Fatal("Deliver() unknown = true, want false")
	}
}
