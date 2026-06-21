package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgrabbitmq "github.com/HappyLadySauce/Beehive-IM/pkg/rabbitmq"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/repository"
)

func TestDispatcherMarksPublishedAfterSuccessfulPublish(t *testing.T) {
	store := &fakeStore{events: []repository.OutboxEvent{{
		EventID:     "event-1",
		RoutingKey:  "message.created.conv-1",
		PayloadJSON: []byte(`{"ok":true}`),
	}}}
	dispatcher := NewDispatcher(Config{BatchSize: 1, PollInterval: time.Hour}, store)
	dispatcher.publisherFactory = func(config pkgrabbitmq.Config) (Publisher, error) {
		return &fakePublisher{}, nil
	}

	if more := dispatcher.dispatchOnce(context.Background()); !more {
		t.Fatal("dispatchOnce() = false, want true when batch is full")
	}
	if len(store.published) != 1 || store.published[0] != "event-1" {
		t.Fatalf("published = %+v, want event-1", store.published)
	}
	if len(store.failed) != 0 {
		t.Fatalf("failed = %+v, want none", store.failed)
	}
}

func TestDispatcherMarksFailedAfterPublishError(t *testing.T) {
	store := &fakeStore{events: []repository.OutboxEvent{{
		EventID:     "event-1",
		RoutingKey:  "message.created.conv-1",
		PayloadJSON: []byte(`{"ok":true}`),
	}}}
	dispatcher := NewDispatcher(Config{BatchSize: 1, PollInterval: time.Hour, RetryBaseDelay: time.Millisecond}, store)
	dispatcher.publisherFactory = func(config pkgrabbitmq.Config) (Publisher, error) {
		return &fakePublisher{err: errors.New("publish failed")}, nil
	}

	dispatcher.dispatchOnce(context.Background())

	if len(store.published) != 0 {
		t.Fatalf("published = %+v, want none", store.published)
	}
	if len(store.failed) != 1 || store.failed[0] != "event-1" {
		t.Fatalf("failed = %+v, want event-1", store.failed)
	}
}

func TestDispatcherRetryDelayCapsAtMax(t *testing.T) {
	dispatcher := NewDispatcher(Config{
		RetryBaseDelay: time.Second,
		RetryMaxDelay:  3 * time.Second,
	}, &fakeStore{})

	if got := dispatcher.retryDelay(10); got != 3*time.Second {
		t.Fatalf("retryDelay() = %v, want 3s", got)
	}
}

type fakeStore struct {
	events    []repository.OutboxEvent
	published []string
	failed    []string
}

func (s *fakeStore) FetchPendingOutbox(ctx context.Context, limit int, lockTTL time.Duration) ([]repository.OutboxEvent, error) {
	events := s.events
	s.events = nil
	return events, nil
}

func (s *fakeStore) MarkOutboxPublished(ctx context.Context, eventID string) error {
	s.published = append(s.published, eventID)
	return nil
}

func (s *fakeStore) MarkOutboxFailed(ctx context.Context, eventID string, err error, maxAttempts int, retryDelay time.Duration) error {
	s.failed = append(s.failed, eventID)
	return nil
}

type fakePublisher struct {
	err error
}

func (p *fakePublisher) Publish(ctx context.Context, routingKey string, payload []byte) error {
	return p.err
}

func (p *fakePublisher) Close() {
}
