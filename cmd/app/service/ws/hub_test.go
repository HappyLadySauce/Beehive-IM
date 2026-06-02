package ws

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type recordingOfflinePublisher struct {
	messages []Envelope
}

func (p *recordingOfflinePublisher) PublishOffline(ctx context.Context, message Envelope) error {
	p.messages = append(p.messages, message)
	return nil
}

func TestHubDeliverMessageToOnlineRecipient(t *testing.T) {
	publisher := &recordingOfflinePublisher{}
	hub := NewHub(publisher)
	recipient := NewClient(ClientIdentity{UserID: "u2", SessionID: "s2"}, nil, 1)
	hub.Register(recipient)
	defer hub.Unregister(recipient)

	err := hub.HandleEnvelope(context.Background(), ClientIdentity{UserID: "u1", SessionID: "s1"}, Envelope{
		ID:   "m1",
		Type: TypeMessageSend,
		Payload: mustMarshal(t, MessageSendPayload{
			ConversationID: "c1",
			ToUserID:       "u2",
			Content:        "hello",
		}),
	})
	if err != nil {
		t.Fatalf("HandleEnvelope returned error: %v", err)
	}

	select {
	case got := <-recipient.Send:
		if got.Type != TypeMessageReceive {
			t.Fatalf("message type = %q, want %q", got.Type, TypeMessageReceive)
		}
		if len(publisher.messages) != 0 {
			t.Fatalf("offline publisher called for online recipient")
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for delivered message")
	}
}

func TestHubPublishesOfflineWhenRecipientIsOffline(t *testing.T) {
	publisher := &recordingOfflinePublisher{}
	hub := NewHub(publisher)

	err := hub.HandleEnvelope(context.Background(), ClientIdentity{UserID: "u1", SessionID: "s1"}, Envelope{
		ID:   "m1",
		Type: TypeMessageSend,
		Payload: mustMarshal(t, MessageSendPayload{
			ConversationID: "c1",
			ToUserID:       "u2",
			Content:        "hello",
		}),
	})
	if err != nil {
		t.Fatalf("HandleEnvelope returned error: %v", err)
	}

	if len(publisher.messages) != 1 {
		t.Fatalf("offline messages = %d, want 1", len(publisher.messages))
	}
	if publisher.messages[0].Type != TypeMessageReceive {
		t.Fatalf("offline message type = %q, want %q", publisher.messages[0].Type, TypeMessageReceive)
	}
}

func TestHubRejectsOfflineMessageWithoutPublisher(t *testing.T) {
	hub := NewHub(nil)

	err := hub.HandleEnvelope(context.Background(), ClientIdentity{UserID: "u1", SessionID: "s1"}, Envelope{
		ID:   "m1",
		Type: TypeMessageSend,
		Payload: mustMarshal(t, MessageSendPayload{
			ConversationID: "c1",
			ToUserID:       "u2",
			Content:        "hello",
		}),
	})
	if err == nil {
		t.Fatalf("HandleEnvelope returned nil, want offline publisher error")
	}
}

func TestHubRejectsInvalidMessagePayload(t *testing.T) {
	hub := NewHub(nil)

	err := hub.HandleEnvelope(context.Background(), ClientIdentity{UserID: "u1", SessionID: "s1"}, Envelope{
		ID:      "m1",
		Type:    TypeMessageSend,
		Payload: json.RawMessage(`{"to_user_id":"","content":""}`),
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
