package ws

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	msgsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/message"
)

type recordingMessageSender struct {
	requests []msgsvc.SendMessageRequest
	err      error
}

func (s *recordingMessageSender) SendMessage(ctx context.Context, sender msgsvc.SenderIdentity, req msgsvc.SendMessageRequest) (msgsvc.StoredMessage, error) {
	s.requests = append(s.requests, req)
	if s.err != nil {
		return msgsvc.StoredMessage{}, s.err
	}
	return msgsvc.StoredMessage{MessageID: "msg_1"}, nil
}

func TestHubDelegatesMessageSendToMessageService(t *testing.T) {
	sender := &recordingMessageSender{}
	hub := NewHub(sender)

	err := hub.HandleEnvelope(context.Background(), ClientIdentity{UserID: "1", SessionID: "s1"}, Envelope{
		ID:   "m1",
		Type: TypeMessageSend,
		Payload: mustMarshal(t, MessageSendPayload{
			ClientMessageID: "client-1",
			ConversationID:  "10",
			Content:         "hello",
		}),
	})
	if err != nil {
		t.Fatalf("HandleEnvelope returned error: %v", err)
	}
	if len(sender.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(sender.requests))
	}
	if sender.requests[0].ClientMessageID != "client-1" {
		t.Fatalf("client_message_id = %q", sender.requests[0].ClientMessageID)
	}
}

func TestHubReturnsMessageServiceError(t *testing.T) {
	hub := NewHub(&recordingMessageSender{err: errors.New("publish failed")})

	err := hub.HandleEnvelope(context.Background(), ClientIdentity{UserID: "1", SessionID: "s1"}, Envelope{
		ID:   "m1",
		Type: TypeMessageSend,
		Payload: mustMarshal(t, MessageSendPayload{
			ClientMessageID: "client-1",
			ConversationID:  "10",
			Content:         "hello",
		}),
	})
	if err == nil {
		t.Fatalf("HandleEnvelope returned nil, want error")
	}
}

func TestHubDeliverToOnlineRecipient(t *testing.T) {
	hub := NewHub(nil)
	recipient := NewClient(ClientIdentity{UserID: "2", SessionID: "s2"}, nil, 1)
	hub.Register(recipient)
	defer hub.Unregister(recipient)

	delivered := hub.DeliverToOnlineUser("2", Envelope{
		ID:   "m1",
		Type: TypeMessageReceive,
		Payload: mustMarshal(t, MessageReceivePayload{
			MessageID:      "m1",
			ConversationID: "10",
			FromUserID:     "1",
			ToUserID:       "2",
			Content:        "hello",
			Sequence:       1,
			SentAt:         time.Now().UnixMilli(),
		}),
	})
	if !delivered {
		t.Fatalf("DeliverToOnlineUser returned false")
	}

	select {
	case got := <-recipient.Send:
		if got.Type != TypeMessageReceive {
			t.Fatalf("message type = %q, want %q", got.Type, TypeMessageReceive)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for delivered message")
	}
}

func TestHubRejectsMessageWithoutSender(t *testing.T) {
	hub := NewHub(nil)

	err := hub.HandleEnvelope(context.Background(), ClientIdentity{UserID: "1", SessionID: "s1"}, Envelope{
		ID:   "m1",
		Type: TypeMessageSend,
		Payload: mustMarshal(t, MessageSendPayload{
			ClientMessageID: "client-1",
			ConversationID:  "10",
			Content:         "hello",
		}),
	})
	if err == nil {
		t.Fatalf("HandleEnvelope returned nil, want message sender error")
	}
}

func TestHubRejectsInvalidMessagePayload(t *testing.T) {
	hub := NewHub(&recordingMessageSender{})

	err := hub.HandleEnvelope(context.Background(), ClientIdentity{UserID: "1", SessionID: "s1"}, Envelope{
		ID:      "m1",
		Type:    TypeMessageSend,
		Payload: json.RawMessage(`{"client_message_id":`),
	})
	if err == nil {
		t.Fatalf("HandleEnvelope returned nil, want validation error")
	}
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}
