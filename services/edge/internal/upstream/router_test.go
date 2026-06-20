package upstream

import (
	"context"
	"testing"

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
