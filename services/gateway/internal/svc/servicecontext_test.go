package svc

import (
	"context"
	"testing"

	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/session"
)

func TestEnterDrainingUpdatesRegistryNodeStatus(t *testing.T) {
	svcCtx := &ServiceContext{
		Config: config.Config{
			UpstreamAddr: "127.0.0.1:9100",
			MaxSessions:  10,
		},
		Sessions: session.NewManager("gateway-1", 10),
		status:   "online",
	}

	if got := svcCtx.registryNode().Status; got != "online" {
		t.Fatalf("initial status = %q, want online", got)
	}
	if err := svcCtx.EnterDraining(context.Background()); err != nil {
		t.Fatalf("EnterDraining() error = %v", err)
	}
	if got := svcCtx.registryNode().Status; got != "draining" {
		t.Fatalf("draining status = %q, want draining", got)
	}
}
