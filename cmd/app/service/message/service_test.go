package message

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	existing      StoredMessage
	existingTo    []string
	existingErr   error
	members       []string
	created       []CreateMessageCommand
	markDelivered []string
	markFailed    []string
}

func (s *fakeStore) GetMessageByClientMessageID(ctx context.Context, senderUserID, clientMessageID string) (StoredMessage, []string, error) {
	if s.existingErr != nil {
		return StoredMessage{}, nil, s.existingErr
	}
	return s.existing, s.existingTo, nil
}

func (s *fakeStore) ListConversationMemberUserIDs(ctx context.Context, conversationID string) ([]string, error) {
	return s.members, nil
}

func (s *fakeStore) CreateMessageWithDeliveries(ctx context.Context, cmd CreateMessageCommand) (StoredMessage, error) {
	s.created = append(s.created, cmd)
	return StoredMessage{
		MessageID:       cmd.MessageID,
		ClientMessageID: cmd.ClientMessageID,
		ConversationID:  cmd.ConversationID,
		FromUserID:      cmd.SenderUserID,
		Content:         cmd.Content,
		Sequence:        1,
		SentAt:          cmd.SentAt,
	}, nil
}

func (s *fakeStore) ListConversationsForUser(ctx context.Context, userID string) ([]StoredConversation, error) {
	return nil, nil
}

func (s *fakeStore) ListMessages(ctx context.Context, userID, conversationID string, beforeSequence uint64, limit int) ([]StoredMessage, error) {
	return nil, nil
}

func (s *fakeStore) MarkDelivered(ctx context.Context, messageID, recipientUserID string) error {
	s.markDelivered = append(s.markDelivered, messageID+":"+recipientUserID)
	return nil
}

func (s *fakeStore) MarkDeliveryFailed(ctx context.Context, messageID, recipientUserID, reason string) error {
	s.markFailed = append(s.markFailed, messageID+":"+recipientUserID)
	return nil
}

type fakePublisher struct {
	events []MessageCreatedEvent
	err    error
}

func (p *fakePublisher) PublishMessageCreated(ctx context.Context, event MessageCreatedEvent) error {
	p.events = append(p.events, event)
	return p.err
}

func TestMessageServiceSendMessageStoresAndPublishes(t *testing.T) {
	store := &fakeStore{
		existingErr: ErrMessageNotFound,
		members:     []string{"1", "2"},
	}
	publisher := &fakePublisher{}
	service := NewMessageService(store, publisher)

	got, err := service.SendMessage(context.Background(), SenderIdentity{UserID: "1"}, SendMessageRequest{
		ClientMessageID: "client-1",
		ConversationID:  "10",
		Content:         " hello ",
	})
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if got.MessageID == "" {
		t.Fatalf("message_id is empty")
	}
	if len(store.created) != 1 {
		t.Fatalf("created messages = %d, want 1", len(store.created))
	}
	if store.created[0].Content != "hello" {
		t.Fatalf("content = %q, want trimmed hello", store.created[0].Content)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.events))
	}
	if publisher.events[0].RecipientUserIDs[0] != "2" {
		t.Fatalf("recipient = %q, want 2", publisher.events[0].RecipientUserIDs[0])
	}
}

func TestMessageServiceRejectsNonMember(t *testing.T) {
	service := NewMessageService(&fakeStore{
		existingErr: ErrMessageNotFound,
		members:     []string{"2", "3"},
	}, &fakePublisher{})

	_, err := service.SendMessage(context.Background(), SenderIdentity{UserID: "1"}, SendMessageRequest{
		ClientMessageID: "client-1",
		ConversationID:  "10",
		Content:         "hello",
	})
	if !errors.Is(err, ErrConversationForbidden) {
		t.Fatalf("error = %v, want ErrConversationForbidden", err)
	}
}

func TestMessageServiceReusesIdempotentMessageWithoutDuplicateInsert(t *testing.T) {
	store := &fakeStore{
		existing: StoredMessage{
			MessageID:       "msg_existing",
			ClientMessageID: "client-1",
			ConversationID:  "10",
			FromUserID:      "1",
			Content:         "hello",
			Sequence:        3,
			SentAt:          time.Now(),
		},
		existingTo: []string{"2"},
	}
	publisher := &fakePublisher{}
	service := NewMessageService(store, publisher)

	got, err := service.SendMessage(context.Background(), SenderIdentity{UserID: "1"}, SendMessageRequest{
		ClientMessageID: "client-1",
		ConversationID:  "10",
		Content:         "hello",
	})
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if got.MessageID != "msg_existing" {
		t.Fatalf("message_id = %q", got.MessageID)
	}
	if len(store.created) != 0 {
		t.Fatalf("created messages = %d, want 0", len(store.created))
	}
	if len(publisher.events) != 1 {
		t.Fatalf("published events = %d, want retry publish", len(publisher.events))
	}
}

func TestMessageServiceReturnsPublishFailure(t *testing.T) {
	service := NewMessageService(&fakeStore{
		existingErr: ErrMessageNotFound,
		members:     []string{"1", "2"},
	}, &fakePublisher{err: errors.New("broker unavailable")})

	_, err := service.SendMessage(context.Background(), SenderIdentity{UserID: "1"}, SendMessageRequest{
		ClientMessageID: "client-1",
		ConversationID:  "10",
		Content:         "hello",
	})
	if err == nil {
		t.Fatalf("SendMessage returned nil, want publish error")
	}
}
