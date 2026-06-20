package logic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/session"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"
)

func TestStreamLogicHandlePing(t *testing.T) {
	manager := session.NewManager("gateway-1", 10)
	if _, err := manager.Attach(&pb.AttachRequest{
		SessionId: "session-1",
		ConnId:    "conn-1",
		EdgeId:    "edge-1",
		UserId:    "user-1",
	}); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}

	logic := NewStreamLogic(context.Background(), &svc.ServiceContext{Sessions: manager})
	resp := logic.handleFrame(&pb.GatewayFrame{
		SessionId: "session-1",
		ConnId:    "conn-1",
		FrameType: "ping",
		ClientSeq: 10,
	})

	if resp.GetFrameType() != "pong" {
		t.Fatalf("FrameType = %q, want pong", resp.GetFrameType())
	}
	if resp.GetServerSeq() != 1 {
		t.Fatalf("ServerSeq = %d, want 1", resp.GetServerSeq())
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(resp.GetPayloadJson()), &payload); err != nil {
		t.Fatalf("payload json error = %v", err)
	}
	if payload["type"] != "pong" {
		t.Fatalf("payload type = %v, want pong", payload["type"])
	}
}

func TestStreamLogicRejectsUnknownSession(t *testing.T) {
	logic := NewStreamLogic(context.Background(), &svc.ServiceContext{
		Sessions: session.NewManager("gateway-1", 10),
	})

	resp := logic.handleFrame(&pb.GatewayFrame{
		SessionId: "missing-session",
		ConnId:    "conn-1",
		FrameType: "ping",
	})
	if resp.GetFrameType() != "error" {
		t.Fatalf("FrameType = %q, want error", resp.GetFrameType())
	}
	if resp.GetServerSeq() != 0 {
		t.Fatalf("ServerSeq = %d, want 0", resp.GetServerSeq())
	}
}
