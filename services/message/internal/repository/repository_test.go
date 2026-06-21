package repository

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestNormalizeSeqsDropsInvalidAndDuplicateValues(t *testing.T) {
	got := normalizeSeqs([]int64{0, 3, 2, 3, -1, 2})
	want := []int64{3, 2}
	if len(got) != len(want) {
		t.Fatalf("normalizeSeqs() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeSeqs() = %+v, want %+v", got, want)
		}
	}
}

func TestNormalizeDirectionDefaultsFromCursor(t *testing.T) {
	if got := normalizeDirection("", 10, 0); got != DirectionForward {
		t.Fatalf("normalizeDirection() = %s, want forward", got)
	}
	if got := normalizeDirection("", 0, 10); got != DirectionBackward {
		t.Fatalf("normalizeDirection() = %s, want backward", got)
	}
	if got := normalizeDirection("forward", 0, 0); got != DirectionForward {
		t.Fatalf("normalizeDirection() = %s, want forward", got)
	}
}

func TestNormalizeLimitCapsMessagePage(t *testing.T) {
	if got := normalizeLimit(0); got != defaultListLimit {
		t.Fatalf("normalizeLimit() = %d, want default", got)
	}
	if got := normalizeLimit(maxListLimit + 1); got != maxListLimit {
		t.Fatalf("normalizeLimit() = %d, want max", got)
	}
}

func TestCodeForErrorMapsInvalidArgument(t *testing.T) {
	err := errors.Join(ErrInvalidArgument, errors.New("field is required"))
	if got := CodeForError(err); got != CodeInvalidArgument {
		t.Fatalf("CodeForError() = %s, want %s", got, CodeInvalidArgument)
	}
}

func TestEnrichOutboxPayloadInjectsEventAndMessageFacts(t *testing.T) {
	payload, err := enrichOutboxPayload([]byte(`{"event_type":"message.created"}`), "event-1", Message{
		MessageID:      "msg-1",
		ConversationID: "conv-1",
		Seq:            7,
		SenderID:       "user-1",
		CreatedAt:      time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("enrichOutboxPayload() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload json error = %v", err)
	}
	if got["event_id"] != "event-1" || got["message_id"] != "msg-1" || got["conversation_id"] != "conv-1" {
		t.Fatalf("payload = %+v", got)
	}
	if got["seq"].(float64) != 7 {
		t.Fatalf("seq = %v, want 7", got["seq"])
	}
}
