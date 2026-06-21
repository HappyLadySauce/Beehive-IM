package logic

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/session"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"
	"github.com/HappyLadySauce/Beehive-IM/services/message/messageservice"
	"google.golang.org/grpc"
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

func TestStreamLogicRejectsMismatchedConn(t *testing.T) {
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
		ConnId:    "conn-2",
		FrameType: "ping",
	})
	if resp.GetFrameType() != "error" {
		t.Fatalf("FrameType = %q, want error", resp.GetFrameType())
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(resp.GetPayloadJson()), &payload); err != nil {
		t.Fatalf("payload json error = %v", err)
	}
	if payload["code"] != session.CodeSessionOwnerMismatch {
		t.Fatalf("payload code = %v, want %s", payload["code"], session.CodeSessionOwnerMismatch)
	}
}

func TestStreamLogicMessageSendCallsMessageService(t *testing.T) {
	manager := session.NewManager("gateway-1", 10)
	if _, err := manager.Attach(&pb.AttachRequest{
		SessionId: "session-1",
		ConnId:    "conn-1",
		EdgeId:    "edge-1",
		UserId:    "user-1",
		DeviceId:  "web-1",
	}); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	fake := &fakeMessageService{
		sendResp: &messageservice.SendMessageResponse{
			Accepted:       true,
			MessageId:      "msg-1",
			ConversationId: "conv-1",
			Seq:            7,
			SenderId:       "user-1",
			ContentType:    "text",
			CreatedAt:      "2026-06-21T00:00:00Z",
		},
	}
	logic := NewStreamLogic(context.Background(), &svc.ServiceContext{Sessions: manager, Message: fake})

	resp := logic.handleFrame(&pb.GatewayFrame{
		SessionId:   "session-1",
		ConnId:      "conn-1",
		FrameType:   "message.send",
		ClientSeq:   11,
		PayloadJson: `{"type":"message.send","seq":11,"payload":{"conversation_id":"conv-1","client_msg_id":"client-1","content_type":"text","content":{"text":"hello"}}}`,
	})

	if resp.GetFrameType() != "message.persisted" {
		t.Fatalf("FrameType = %q, want message.persisted", resp.GetFrameType())
	}
	if fake.sendReq.GetSenderId() != "user-1" || fake.sendReq.GetDeviceId() != "web-1" {
		t.Fatalf("send request identity = %s/%s, want user-1/web-1", fake.sendReq.GetSenderId(), fake.sendReq.GetDeviceId())
	}
	if fake.sendReq.GetContentJson() != `{"text":"hello"}` {
		t.Fatalf("ContentJson = %q", fake.sendReq.GetContentJson())
	}
}

func TestStreamLogicMessageSendRPCErrorReturnsErrorFrame(t *testing.T) {
	manager := session.NewManager("gateway-1", 10)
	if _, err := manager.Attach(&pb.AttachRequest{
		SessionId: "session-1",
		ConnId:    "conn-1",
		EdgeId:    "edge-1",
		UserId:    "user-1",
		DeviceId:  "web-1",
	}); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	logic := NewStreamLogic(context.Background(), &svc.ServiceContext{
		Sessions: manager,
		Message:  &fakeMessageService{sendErr: errors.New("rpc unavailable")},
	})

	resp := logic.handleFrame(&pb.GatewayFrame{
		SessionId:   "session-1",
		ConnId:      "conn-1",
		FrameType:   "message.send",
		ClientSeq:   12,
		PayloadJson: `{"type":"message.send","seq":12,"payload":{"conversation_id":"conv-1","client_msg_id":"client-1","content":{"text":"hello"}}}`,
	})

	assertErrorCode(t, resp, "MESSAGE_SEND_FAILED")
}

func TestStreamLogicMessageAckCallsMessageService(t *testing.T) {
	manager := session.NewManager("gateway-1", 10)
	if _, err := manager.Attach(&pb.AttachRequest{
		SessionId: "session-1",
		ConnId:    "conn-1",
		EdgeId:    "edge-1",
		UserId:    "user-1",
		DeviceId:  "web-1",
	}); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	fake := &fakeMessageService{ackResp: &messageservice.AckMessagesResponse{Accepted: true, Updated: 2}}
	logic := NewStreamLogic(context.Background(), &svc.ServiceContext{Sessions: manager, Message: fake})

	resp := logic.handleFrame(&pb.GatewayFrame{
		SessionId:   "session-1",
		ConnId:      "conn-1",
		FrameType:   "message.ack",
		ClientSeq:   13,
		PayloadJson: `{"type":"message.ack","seq":13,"payload":{"conversation_id":"conv-1","ack_type":"read","seqs":[1,2]}}`,
	})

	if resp.GetFrameType() != "message.ack" {
		t.Fatalf("FrameType = %q, want message.ack", resp.GetFrameType())
	}
	if fake.ackReq.GetUserId() != "user-1" || fake.ackReq.GetDeviceId() != "web-1" {
		t.Fatalf("ack request identity = %s/%s, want user-1/web-1", fake.ackReq.GetUserId(), fake.ackReq.GetDeviceId())
	}
}

type fakeMessageService struct {
	sendReq  *messageservice.SendMessageRequest
	sendResp *messageservice.SendMessageResponse
	sendErr  error
	ackReq   *messageservice.AckMessagesRequest
	ackResp  *messageservice.AckMessagesResponse
	ackErr   error
}

func (f *fakeMessageService) SendMessage(ctx context.Context, in *messageservice.SendMessageRequest, opts ...grpc.CallOption) (*messageservice.SendMessageResponse, error) {
	f.sendReq = in
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	if f.sendResp != nil {
		return f.sendResp, nil
	}
	return &messageservice.SendMessageResponse{Accepted: true}, nil
}

func (f *fakeMessageService) AckMessages(ctx context.Context, in *messageservice.AckMessagesRequest, opts ...grpc.CallOption) (*messageservice.AckMessagesResponse, error) {
	f.ackReq = in
	if f.ackErr != nil {
		return nil, f.ackErr
	}
	if f.ackResp != nil {
		return f.ackResp, nil
	}
	return &messageservice.AckMessagesResponse{Accepted: true}, nil
}

func assertErrorCode(t *testing.T, frame *pb.GatewayFrame, want string) {
	t.Helper()

	if frame.GetFrameType() != "error" {
		t.Fatalf("FrameType = %q, want error", frame.GetFrameType())
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(frame.GetPayloadJson()), &payload); err != nil {
		t.Fatalf("payload json error = %v", err)
	}
	if payload["code"] != want {
		t.Fatalf("payload code = %v, want %s", payload["code"], want)
	}
}
