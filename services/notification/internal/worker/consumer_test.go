package worker

import (
	"encoding/json"
	"testing"

	"github.com/HappyLadySauce/Beehive-IM/services/presence/presenceservice"
)

func TestDecodeMessageCreatedEventRequiresFacts(t *testing.T) {
	event, err := decodeMessageCreatedEvent([]byte(`{
		"event_id":"event-1",
		"event_type":"message.created",
		"message_id":"msg-1",
		"conversation_id":"conv-1",
		"seq":7,
		"sender_id":"user-1",
		"device_id":"web-1",
		"content_type":"text",
		"content":{"text":"hello"}
	}`))
	if err != nil {
		t.Fatalf("decodeMessageCreatedEvent() error = %v", err)
	}
	if event.EventID != "event-1" || event.Seq != 7 || event.SenderID != "user-1" {
		t.Fatalf("event = %+v", event)
	}
}

func TestDecodeMessageCreatedEventRejectsMalformedFacts(t *testing.T) {
	if _, err := decodeMessageCreatedEvent([]byte(`{"event_id":"event-1"}`)); err == nil {
		t.Fatal("decodeMessageCreatedEvent() error = nil, want required fields error")
	}
}

func TestBuildEdgePushUsesMessageNewEnvelope(t *testing.T) {
	body, err := buildEdgePush(MessageCreatedEvent{
		EventID:        "event-1",
		MessageID:      "msg-1",
		ConversationID: "conv-1",
		Seq:            7,
		SenderID:       "user-1",
		DeviceID:       "web-1",
		ClientMsgID:    "client-1",
		ContentType:    "text",
		Content:        json.RawMessage(`{"text":"hello"}`),
		CreatedAt:      "2026-06-21T00:00:00Z",
	}, &presenceservice.ConnectionMeta{
		ConnId:    "conn-1",
		SessionId: "session-1",
		EdgeId:    "edge-1",
		UserId:    "user-2",
		DeviceId:  "web-2",
	})
	if err != nil {
		t.Fatalf("buildEdgePush() error = %v", err)
	}
	var envelope edgePushMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if envelope.Type != "message.new" || envelope.ConnID != "conn-1" || envelope.SessionID != "session-1" {
		t.Fatalf("envelope = %+v", envelope)
	}
	var payload map[string]any
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("payload json error = %v", err)
	}
	if payload["event_id"] != "event-1" || payload["message_id"] != "msg-1" || payload["conversation_id"] != "conv-1" {
		t.Fatalf("payload = %+v", payload)
	}
	if payload["seq"].(float64) != 7 {
		t.Fatalf("seq = %v, want 7", payload["seq"])
	}
}
