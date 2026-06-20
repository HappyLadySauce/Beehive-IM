package upstream

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgetcd "github.com/HappyLadySauce/Beehive-IM/pkg/etcd"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/gatewayservice"
)

type fakeGatewayService struct {
	gatewayservice.GatewayService
}

func TestRouterPicksLeastLoadedHealthyNode(t *testing.T) {
	router := &Router{
		nodes: []pkgetcd.ServiceNode{
			{InstanceID: "gw-busy", Address: "127.0.0.1:9101", Status: "online", SessionCount: 10, MaxSessions: 20},
			{InstanceID: "gw-free", Address: "127.0.0.1:9102", Status: "online", SessionCount: 1, MaxSessions: 20},
		},
	}
	node, ok := router.pickNode()
	if !ok {
		t.Fatal("pickNode() ok = false, want true")
	}
	if node.InstanceID != "gw-free" {
		t.Fatalf("InstanceID = %q, want gw-free", node.InstanceID)
	}
}

func TestRouterSkipsExcludedGateway(t *testing.T) {
	router := &Router{
		nodes: []pkgetcd.ServiceNode{
			{InstanceID: "gw-a", Address: "127.0.0.1:9101", Status: "online", SessionCount: 1, MaxSessions: 20},
			{InstanceID: "gw-b", Address: "127.0.0.1:9102", Status: "online", SessionCount: 10, MaxSessions: 20},
		},
	}
	node, ok := router.pickNode("gw-a")
	if !ok {
		t.Fatal("pickNode() ok = false, want true")
	}
	if node.InstanceID != "gw-b" {
		t.Fatalf("InstanceID = %q, want gw-b", node.InstanceID)
	}
}

func TestRouterSkipsDrainingAndIsolatedGateways(t *testing.T) {
	router := &Router{
		nodes: []pkgetcd.ServiceNode{
			{InstanceID: "gw-draining", Address: "127.0.0.1:9101", Status: StatusDraining, SessionCount: 1, MaxSessions: 20},
			{InstanceID: "gw-isolated", Address: "127.0.0.1:9102", Status: StatusOnline, SessionCount: 1, MaxSessions: 20},
			{InstanceID: "gw-ok", Address: "127.0.0.1:9103", Status: StatusOnline, SessionCount: 10, MaxSessions: 20},
		},
		isolatedUntil: map[string]time.Time{"gw-isolated": time.Now().Add(time.Minute)},
	}

	node, ok := router.pickNode()
	if !ok {
		t.Fatal("pickNode() ok = false, want true")
	}
	if node.InstanceID != "gw-ok" {
		t.Fatalf("InstanceID = %q, want gw-ok", node.InstanceID)
	}
}

func TestRouterAllowsGatewayAfterIsolationExpires(t *testing.T) {
	router := &Router{
		nodes: []pkgetcd.ServiceNode{
			{InstanceID: "gw-expired", Address: "127.0.0.1:9101", Status: StatusOnline, SessionCount: 1, MaxSessions: 20},
			{InstanceID: "gw-busy", Address: "127.0.0.1:9102", Status: StatusOnline, SessionCount: 10, MaxSessions: 20},
		},
		isolatedUntil: map[string]time.Time{"gw-expired": time.Now().Add(-time.Second)},
	}

	node, ok := router.pickNode()
	if !ok {
		t.Fatal("pickNode() ok = false, want true")
	}
	if node.InstanceID != "gw-expired" {
		t.Fatalf("InstanceID = %q, want gw-expired", node.InstanceID)
	}
}

func TestRouterFallsBackToStaticGateway(t *testing.T) {
	static := fakeGatewayService{}
	router := &Router{static: static, staticID: "static-gateway"}

	got, id, err := router.Pick(context.Background())
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}
	if got == nil || id != "static-gateway" {
		t.Fatalf("Pick() = (%v, %q), want static gateway", got, id)
	}
}

func TestRouterDoesNotFallbackToExcludedStaticGateway(t *testing.T) {
	static := fakeGatewayService{}
	router := &Router{static: static, staticID: "static-gateway"}

	_, _, err := router.Pick(context.Background(), "static-gateway")
	if !errors.Is(err, ErrNoGatewayAvailable) {
		t.Fatalf("Pick() error = %v, want %v", err, ErrNoGatewayAvailable)
	}
}

func TestRouterDoesNotFallbackToIsolatedStaticGateway(t *testing.T) {
	static := fakeGatewayService{}
	router := &Router{
		static:        static,
		staticID:      "static-gateway",
		isolatedUntil: map[string]time.Time{"static-gateway": time.Now().Add(time.Minute)},
	}

	_, _, err := router.Pick(context.Background())
	if !errors.Is(err, ErrNoGatewayAvailable) {
		t.Fatalf("Pick() error = %v, want %v", err, ErrNoGatewayAvailable)
	}
}

func TestRouterGatewayEventsForDrainingAndDeleted(t *testing.T) {
	router := &Router{
		nodes: []pkgetcd.ServiceNode{
			{InstanceID: "gw-draining", Status: StatusOnline},
			{InstanceID: "gw-deleted", Status: StatusOnline},
		},
	}

	events := router.gatewayEvents([]pkgetcd.ServiceNode{
		{InstanceID: "gw-draining", Status: StatusDraining},
	}, true)
	if len(events) != 2 {
		t.Fatalf("events len = %d, want 2", len(events))
	}

	got := map[string]string{}
	for _, event := range events {
		got[event.GatewayID] = event.Status
	}
	if got["gw-draining"] != StatusDraining {
		t.Fatalf("gw-draining status = %q, want %s", got["gw-draining"], StatusDraining)
	}
	if got["gw-deleted"] != StatusDeleted {
		t.Fatalf("gw-deleted status = %q, want %s", got["gw-deleted"], StatusDeleted)
	}
}
