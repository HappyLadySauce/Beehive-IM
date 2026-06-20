package logic

import (
	"context"
	"testing"

	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/session"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"
)

func TestAttachRejectsWhenGatewayDraining(t *testing.T) {
	svcCtx := &svc.ServiceContext{Sessions: session.NewManager("gateway-1", 10)}
	if err := svcCtx.EnterDraining(context.Background()); err != nil {
		t.Fatalf("EnterDraining() error = %v", err)
	}

	resp, err := NewAttachLogic(context.Background(), svcCtx).Attach(&pb.AttachRequest{
		SessionId: "session-1",
		ConnId:    "conn-1",
		EdgeId:    "edge-1",
		UserId:    "user-1",
	})
	if err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	if resp.GetAccepted() {
		t.Fatal("Attach() accepted = true, want false")
	}
	if resp.GetErrorCode() != session.CodeGatewayDraining {
		t.Fatalf("ErrorCode = %q, want %s", resp.GetErrorCode(), session.CodeGatewayDraining)
	}
}

func TestResumeRejectsWhenGatewayDraining(t *testing.T) {
	svcCtx := &svc.ServiceContext{Sessions: session.NewManager("gateway-1", 10)}
	if err := svcCtx.EnterDraining(context.Background()); err != nil {
		t.Fatalf("EnterDraining() error = %v", err)
	}

	resp, err := NewResumeLogic(context.Background(), svcCtx).Resume(&pb.ResumeRequest{
		SessionId: "session-1",
		ConnId:    "conn-1",
		EdgeId:    "edge-1",
		UserId:    "user-1",
	})
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if resp.GetAccepted() {
		t.Fatal("Resume() accepted = true, want false")
	}
	if resp.GetErrorCode() != session.CodeGatewayDraining {
		t.Fatalf("ErrorCode = %q, want %s", resp.GetErrorCode(), session.CodeGatewayDraining)
	}
}
