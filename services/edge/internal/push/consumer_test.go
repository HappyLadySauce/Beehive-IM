package push

import (
	"context"
	"encoding/json"
	"testing"

	pkgrabbitmq "github.com/HappyLadySauce/Beehive-IM/pkg/rabbitmq"
)

func TestMessageMarshalShape(t *testing.T) {
	body, err := json.Marshal(Message{
		ConnID:    "conn-1",
		SessionID: "session-1",
		Type:      "message.new",
		Payload:   json.RawMessage(`{"id":"msg-1"}`),
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded Message
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.Type != "message.new" || decoded.ConnID != "conn-1" {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func TestFramePayloadDefaultsEmptyPayload(t *testing.T) {
	frame, err := FramePayload(Message{})
	if err != nil {
		t.Fatalf("FramePayload() error = %v", err)
	}
	if string(frame) != `{"payload":{},"type":"push"}` {
		t.Fatalf("frame = %s", frame)
	}
}

func TestConsumerStopIsIdempotent(t *testing.T) {
	consumer := NewConsumer("edge-1", zeroRabbitConfig(), nil)
	consumer.Stop()
	consumer.Stop()
	consumer.Start(context.Background())
}

func zeroRabbitConfig() pkgrabbitmq.Config {
	return pkgrabbitmq.Config{}
}
