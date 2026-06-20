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

func TestHubMigrateGatewaySendsOnlyMatchingConnections(t *testing.T) {
	hub := NewHub()
	outboundA := make(chan []byte, 1)
	outboundB := make(chan []byte, 1)
	controlA := make(chan controlMessage, 1)
	controlB := make(chan controlMessage, 1)
	hub.RegisterConnection("conn-a", "session-a", "gateway-a", outboundA, controlA)
	hub.RegisterConnection("conn-b", "session-b", "gateway-b", outboundB, controlB)
	defer hub.UnregisterConnection("conn-a", "session-a")
	defer hub.UnregisterConnection("conn-b", "session-b")

	if got := hub.MigrateGateway("gateway-a", "draining"); got != 1 {
		t.Fatalf("MigrateGateway() = %d, want 1", got)
	}

	select {
	case msg := <-controlA:
		if msg.Kind != controlMigrateGateway || msg.GatewayID != "gateway-a" {
			t.Fatalf("control message = %+v, want gateway-a migrate", msg)
		}
	default:
		t.Fatal("controlA did not receive migrate message")
	}
	select {
	case msg := <-controlB:
		t.Fatalf("controlB received unexpected message: %+v", msg)
	default:
	}
}

func TestHubUpdateGatewayMovesConnectionIndex(t *testing.T) {
	hub := NewHub()
	outbound := make(chan []byte, 1)
	control := make(chan controlMessage, 2)
	hub.RegisterConnection("conn-a", "session-a", "gateway-a", outbound, control)
	defer hub.UnregisterConnection("conn-a", "session-a")

	hub.UpdateGateway("conn-a", "gateway-b")
	if got := hub.MigrateGateway("gateway-a", "draining"); got != 0 {
		t.Fatalf("MigrateGateway(gateway-a) = %d, want 0", got)
	}
	if got := hub.MigrateGateway("gateway-b", "draining"); got != 1 {
		t.Fatalf("MigrateGateway(gateway-b) = %d, want 1", got)
	}
}
