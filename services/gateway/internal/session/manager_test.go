package session

import (
	"errors"
	"testing"

	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"
)

func TestManagerAttachCreatesSession(t *testing.T) {
	manager := NewManager("gateway-1", 10)

	sess, err := manager.Attach(&pb.AttachRequest{
		SessionId: "session-1",
		ConnId:    "conn-1",
		EdgeId:    "edge-1",
		UserId:    "user-1",
		DeviceId:  "device-1",
	})
	if err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	if sess.SessionID != "session-1" || sess.ConnID != "conn-1" || sess.EdgeID != "edge-1" {
		t.Fatalf("Attach() returned unexpected session: %+v", sess)
	}
	if !manager.Exists("session-1") {
		t.Fatal("Exists() = false, want true")
	}
}

func TestManagerAttachRejectsDuplicateSession(t *testing.T) {
	manager := NewManager("gateway-1", 10)
	req := &pb.AttachRequest{
		SessionId: "session-1",
		ConnId:    "conn-1",
		EdgeId:    "edge-1",
		UserId:    "user-1",
	}

	if _, err := manager.Attach(req); err != nil {
		t.Fatalf("Attach() first error = %v", err)
	}
	_, err := manager.Attach(req)
	if !errors.Is(err, ErrSessionAlreadyExists) {
		t.Fatalf("Attach() duplicate error = %v, want %v", err, ErrSessionAlreadyExists)
	}
}

func TestManagerRejectsSessionCapacity(t *testing.T) {
	manager := NewManager("gateway-1", 1)
	if _, err := manager.Attach(&pb.AttachRequest{
		SessionId: "session-1",
		ConnId:    "conn-1",
		EdgeId:    "edge-1",
		UserId:    "user-1",
	}); err != nil {
		t.Fatalf("Attach() first error = %v", err)
	}

	_, err := manager.Attach(&pb.AttachRequest{
		SessionId: "session-2",
		ConnId:    "conn-2",
		EdgeId:    "edge-1",
		UserId:    "user-1",
	})
	if !errors.Is(err, ErrSessionCapacity) {
		t.Fatalf("Attach() capacity error = %v, want %v", err, ErrSessionCapacity)
	}
}

func TestManagerResumeUpdatesExistingSession(t *testing.T) {
	manager := NewManager("gateway-1", 10)
	if _, err := manager.Attach(&pb.AttachRequest{
		SessionId: "session-1",
		ConnId:    "conn-1",
		EdgeId:    "edge-1",
		UserId:    "user-1",
	}); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}

	sess, err := manager.Resume(&pb.ResumeRequest{
		SessionId:        "session-1",
		ConnId:           "conn-2",
		EdgeId:           "edge-2",
		UserId:           "user-1",
		LastClientSeq:    7,
		LastDeliveredSeq: 5,
	})
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if sess.ConnID != "conn-2" || sess.EdgeID != "edge-2" || sess.LastClientSeq != 7 || sess.LastDeliveredSeq != 5 {
		t.Fatalf("Resume() returned unexpected session: %+v", sess)
	}
}

func TestManagerCloseRequiresMatchingOwner(t *testing.T) {
	manager := NewManager("gateway-1", 10)
	if _, err := manager.Attach(&pb.AttachRequest{
		SessionId: "session-1",
		ConnId:    "conn-1",
		EdgeId:    "edge-1",
		UserId:    "user-1",
	}); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}

	if manager.Close(&pb.CloseSessionRequest{SessionId: "session-1", ConnId: "conn-2", EdgeId: "edge-1"}) {
		t.Fatal("Close() with mismatched conn = true, want false")
	}
	if !manager.Exists("session-1") {
		t.Fatal("session removed after mismatched close")
	}
	if !manager.Close(&pb.CloseSessionRequest{SessionId: "session-1", ConnId: "conn-1", EdgeId: "edge-1"}) {
		t.Fatal("Close() = false, want true")
	}
	if manager.Exists("session-1") {
		t.Fatal("session still exists after close")
	}
}
