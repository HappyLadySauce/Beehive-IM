package message

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidMessage        = errors.New("invalid message")
	ErrConversationForbidden = errors.New("conversation access denied")
)

// Store defines the persistence contract used by MessageService and dispatchers.
// Store 定义 MessageService 与投递调度器使用的持久化契约。
type Store interface {
	GetMessageByClientMessageID(ctx context.Context, senderUserID, clientMessageID string) (StoredMessage, []string, error)
	ListConversationMemberUserIDs(ctx context.Context, conversationID string) ([]string, error)
	CreateMessageWithDeliveries(ctx context.Context, cmd CreateMessageCommand) (StoredMessage, error)
	ListConversationsForUser(ctx context.Context, userID string) ([]StoredConversation, error)
	ListMessages(ctx context.Context, userID, conversationID string, beforeSequence uint64, limit int) ([]StoredMessage, error)
	MarkDelivered(ctx context.Context, messageID, recipientUserID string) error
	MarkDeliveryFailed(ctx context.Context, messageID, recipientUserID, reason string) error
}

// Publisher publishes message lifecycle events to the broker.
// Publisher 将消息生命周期事件发布到消息代理。
type Publisher interface {
	PublishMessageCreated(ctx context.Context, event MessageCreatedEvent) error
}

// MessageService validates and persists user messages before broker publication.
// MessageService 在发布到消息代理前校验并持久化用户消息。
type MessageService struct {
	store     Store
	publisher Publisher
}

func NewMessageService(store Store, publisher Publisher) *MessageService {
	return &MessageService{
		store:     store,
		publisher: publisher,
	}
}

// SendMessage validates membership, writes the message, and publishes a broker event.
// SendMessage 校验成员权限、写入消息并发布消息代理事件。
func (s *MessageService) SendMessage(ctx context.Context, sender SenderIdentity, req SendMessageRequest) (StoredMessage, error) {
	if ctx == nil {
		return StoredMessage{}, fmt.Errorf("context is nil")
	}
	if s == nil || s.store == nil || s.publisher == nil {
		return StoredMessage{}, fmt.Errorf("message service is not fully initialized")
	}

	normalized, err := normalizeRequest(sender, req)
	if err != nil {
		return StoredMessage{}, err
	}

	existing, recipients, err := s.store.GetMessageByClientMessageID(ctx, sender.UserID, normalized.ClientMessageID)
	if err == nil {
		if err := s.publishStoredMessage(ctx, existing, recipients); err != nil {
			return StoredMessage{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, ErrMessageNotFound) {
		return StoredMessage{}, err
	}

	members, err := s.store.ListConversationMemberUserIDs(ctx, normalized.ConversationID)
	if err != nil {
		return StoredMessage{}, err
	}
	if !containsUser(members, sender.UserID) {
		return StoredMessage{}, ErrConversationForbidden
	}

	recipients = make([]string, 0, len(members)-1)
	for _, memberID := range members {
		if memberID != sender.UserID {
			recipients = append(recipients, memberID)
		}
	}
	if len(recipients) == 0 {
		return StoredMessage{}, fmt.Errorf("%w: conversation has no recipient", ErrInvalidMessage)
	}

	messageID, err := generateMessageID()
	if err != nil {
		return StoredMessage{}, err
	}
	stored, err := s.store.CreateMessageWithDeliveries(ctx, CreateMessageCommand{
		MessageID:        messageID,
		ClientMessageID:  normalized.ClientMessageID,
		ConversationID:   normalized.ConversationID,
		SenderUserID:     sender.UserID,
		RecipientUserIDs: recipients,
		Content:          normalized.Content,
		SentAt:           time.Now().UTC(),
	})
	if err != nil {
		return StoredMessage{}, err
	}

	if err := s.publishStoredMessage(ctx, stored, recipients); err != nil {
		return StoredMessage{}, err
	}
	return stored, nil
}

func (s *MessageService) publishStoredMessage(ctx context.Context, stored StoredMessage, recipients []string) error {
	event := MessageCreatedEvent{
		MessageID:        stored.MessageID,
		ClientMessageID:  stored.ClientMessageID,
		ConversationID:   stored.ConversationID,
		FromUserID:       stored.FromUserID,
		RecipientUserIDs: recipients,
		Content:          stored.Content,
		Sequence:         stored.Sequence,
		SentAt:           stored.SentAt.UnixMilli(),
	}
	if err := s.publisher.PublishMessageCreated(ctx, event); err != nil {
		return fmt.Errorf("publish message created event: %w", err)
	}
	return nil
}

func normalizeRequest(sender SenderIdentity, req SendMessageRequest) (SendMessageRequest, error) {
	if strings.TrimSpace(sender.UserID) == "" {
		return SendMessageRequest{}, fmt.Errorf("%w: sender user_id is required", ErrInvalidMessage)
	}
	req.ClientMessageID = strings.TrimSpace(req.ClientMessageID)
	req.ConversationID = strings.TrimSpace(req.ConversationID)
	req.Content = strings.TrimSpace(req.Content)
	if req.ClientMessageID == "" {
		return SendMessageRequest{}, fmt.Errorf("%w: client_message_id is required", ErrInvalidMessage)
	}
	if req.ConversationID == "" {
		return SendMessageRequest{}, fmt.Errorf("%w: conversation_id is required", ErrInvalidMessage)
	}
	if req.Content == "" {
		return SendMessageRequest{}, fmt.Errorf("%w: content is required", ErrInvalidMessage)
	}
	if len([]byte(req.Content)) > maxTextContentBytes {
		return SendMessageRequest{}, fmt.Errorf("%w: content exceeds %d bytes", ErrInvalidMessage, maxTextContentBytes)
	}
	return req, nil
}

func containsUser(userIDs []string, target string) bool {
	for _, userID := range userIDs {
		if userID == target {
			return true
		}
	}
	return false
}

func generateMessageID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate message id: %w", err)
	}
	return "msg_" + hex.EncodeToString(buf[:]), nil
}
