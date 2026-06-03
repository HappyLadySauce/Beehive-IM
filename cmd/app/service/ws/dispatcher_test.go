package ws

import (
	"context"
	"testing"
	"time"

	msgsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/message"
)

type dispatcherStore struct {
	delivered []string
	failed    []string
}

func (s *dispatcherStore) GetMessageByClientMessageID(ctx context.Context, senderUserID, clientMessageID string) (msgsvc.StoredMessage, []string, error) {
	return msgsvc.StoredMessage{}, nil, msgsvc.ErrMessageNotFound
}

func (s *dispatcherStore) ListConversationMemberUserIDs(ctx context.Context, conversationID string) ([]string, error) {
	return nil, nil
}

func (s *dispatcherStore) CreateMessageWithDeliveries(ctx context.Context, cmd msgsvc.CreateMessageCommand) (msgsvc.StoredMessage, error) {
	return msgsvc.StoredMessage{}, nil
}

func (s *dispatcherStore) ListConversationsForUser(ctx context.Context, userID string) ([]msgsvc.StoredConversation, error) {
	return nil, nil
}

func (s *dispatcherStore) ListMessages(ctx context.Context, userID, conversationID string, beforeSequence uint64, limit int) ([]msgsvc.StoredMessage, error) {
	return nil, nil
}

func (s *dispatcherStore) MarkDelivered(ctx context.Context, messageID, recipientUserID string) error {
	s.delivered = append(s.delivered, messageID+":"+recipientUserID)
	return nil
}

func (s *dispatcherStore) MarkDeliveryFailed(ctx context.Context, messageID, recipientUserID, reason string) error {
	s.failed = append(s.failed, messageID+":"+recipientUserID)
	return nil
}

func TestDispatcherDeliversOnlineRecipientAndMarksDelivered(t *testing.T) {
	store := &dispatcherStore{}
	hub := NewHub(nil)
	recipient := NewClient(ClientIdentity{UserID: "2"}, nil, 1)
	hub.Register(recipient)
	defer hub.Unregister(recipient)

	dispatcher := NewMessageDispatcher(store, hub)
	err := dispatcher.HandleMessageCreated(context.Background(), msgsvc.MessageCreatedEvent{
		MessageID:        "msg_1",
		ClientMessageID:  "client-1",
		ConversationID:   "10",
		FromUserID:       "1",
		RecipientUserIDs: []string{"2"},
		Content:          "hello",
		Sequence:         1,
		SentAt:           time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatalf("HandleMessageCreated returned error: %v", err)
	}
	if len(store.delivered) != 1 || store.delivered[0] != "msg_1:2" {
		t.Fatalf("delivered = %#v", store.delivered)
	}
	select {
	case got := <-recipient.Send:
		if got.Type != TypeMessageReceive {
			t.Fatalf("type = %q", got.Type)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for websocket message")
	}
}

func TestDispatcherMarksOfflineRecipientFailed(t *testing.T) {
	store := &dispatcherStore{}
	dispatcher := NewMessageDispatcher(store, NewHub(nil))

	err := dispatcher.HandleMessageCreated(context.Background(), msgsvc.MessageCreatedEvent{
		MessageID:        "msg_1",
		ConversationID:   "10",
		FromUserID:       "1",
		RecipientUserIDs: []string{"2"},
		Content:          "hello",
		Sequence:         1,
		SentAt:           time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatalf("HandleMessageCreated returned error: %v", err)
	}
	if len(store.failed) != 1 || store.failed[0] != "msg_1:2" {
		t.Fatalf("failed = %#v", store.failed)
	}
}
