package message

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/model"
)

var ErrMessageNotFound = errors.New("message not found")

// GormStore persists conversations, messages, and delivery state with GORM.
// GormStore 使用 GORM 持久化会话、消息与投递状态。
type GormStore struct {
	db *gorm.DB
}

func NewGormStore(db *gorm.DB) *GormStore {
	return &GormStore{db: db}
}

// GetMessageByClientMessageID returns an existing idempotent message and recipients.
// GetMessageByClientMessageID 返回已存在的幂等消息及接收人列表。
func (s *GormStore) GetMessageByClientMessageID(ctx context.Context, senderUserID, clientMessageID string) (StoredMessage, []string, error) {
	if s == nil || s.db == nil {
		return StoredMessage{}, nil, fmt.Errorf("message store is not initialized")
	}
	senderID, err := parseUintID(senderUserID, "sender_user_id")
	if err != nil {
		return StoredMessage{}, nil, err
	}

	var row model.Message
	err = s.db.WithContext(ctx).
		Where("sender_user_id = ? AND client_message_id = ?", senderID, clientMessageID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return StoredMessage{}, nil, ErrMessageNotFound
	}
	if err != nil {
		return StoredMessage{}, nil, fmt.Errorf("get idempotent message: %w", err)
	}

	recipients, err := s.listRecipientUserIDs(ctx, row.MessageID)
	if err != nil {
		return StoredMessage{}, nil, err
	}
	return toStoredMessage(row), recipients, nil
}

// ListConversationMemberUserIDs returns active member user IDs for one conversation.
// ListConversationMemberUserIDs 返回指定会话的有效成员用户 ID。
func (s *GormStore) ListConversationMemberUserIDs(ctx context.Context, conversationID string) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("message store is not initialized")
	}
	convID, err := parseUintID(conversationID, "conversation_id")
	if err != nil {
		return nil, err
	}

	var members []model.ConversationMember
	if err := s.db.WithContext(ctx).
		Where("conversation_id = ?", convID).
		Order("user_id ASC").
		Find(&members).Error; err != nil {
		return nil, fmt.Errorf("list conversation members: %w", err)
	}
	userIDs := make([]string, 0, len(members))
	for _, member := range members {
		userIDs = append(userIDs, strconv.FormatUint(member.UserID, 10))
	}
	return userIDs, nil
}

// CreateMessageWithDeliveries stores a message and pending delivery rows atomically.
// CreateMessageWithDeliveries 原子写入消息与待投递记录。
func (s *GormStore) CreateMessageWithDeliveries(ctx context.Context, cmd CreateMessageCommand) (StoredMessage, error) {
	if s == nil || s.db == nil {
		return StoredMessage{}, fmt.Errorf("message store is not initialized")
	}
	conversationID, err := parseUintID(cmd.ConversationID, "conversation_id")
	if err != nil {
		return StoredMessage{}, err
	}
	senderID, err := parseUintID(cmd.SenderUserID, "sender_user_id")
	if err != nil {
		return StoredMessage{}, err
	}
	recipientIDs := make([]uint64, 0, len(cmd.RecipientUserIDs))
	for _, recipient := range cmd.RecipientUserIDs {
		recipientID, err := parseUintID(recipient, "recipient_user_id")
		if err != nil {
			return StoredMessage{}, err
		}
		recipientIDs = append(recipientIDs, recipientID)
	}

	var stored model.Message
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sequence, err := nextConversationSequence(ctx, tx, conversationID)
		if err != nil {
			return err
		}
		stored = model.Message{
			MessageID:       cmd.MessageID,
			ConversationID:  conversationID,
			SenderUserID:    senderID,
			ClientMessageID: cmd.ClientMessageID,
			Content:         cmd.Content,
			Status:          model.MessageStatusCreated,
			Sequence:        sequence,
			SentAt:          cmd.SentAt,
		}
		if stored.SentAt.IsZero() {
			stored.SentAt = time.Now().UTC()
		}
		if err := tx.Create(&stored).Error; err != nil {
			return fmt.Errorf("create message: %w", err)
		}

		deliveries := make([]model.MessageDelivery, 0, len(recipientIDs))
		for _, recipientID := range recipientIDs {
			deliveries = append(deliveries, model.MessageDelivery{
				MessageID:       stored.MessageID,
				ConversationID:  conversationID,
				RecipientUserID: recipientID,
				Status:          model.DeliveryStatusPending,
			})
		}
		if len(deliveries) > 0 {
			if err := tx.Create(&deliveries).Error; err != nil {
				return fmt.Errorf("create message deliveries: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return StoredMessage{}, err
	}
	return toStoredMessage(stored), nil
}

// ListConversationsForUser returns conversations visible to one user.
// ListConversationsForUser 返回指定用户可见的会话列表。
func (s *GormStore) ListConversationsForUser(ctx context.Context, userID string) ([]StoredConversation, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("message store is not initialized")
	}
	uid, err := parseUintID(userID, "user_id")
	if err != nil {
		return nil, err
	}

	var rows []model.Conversation
	err = s.db.WithContext(ctx).
		Joins("JOIN conversation_members cm ON cm.conversation_id = conversations.id AND cm.deleted_at IS NULL").
		Where("cm.user_id = ?", uid).
		Order("conversations.updated_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}

	items := make([]StoredConversation, 0, len(rows))
	for _, row := range rows {
		items = append(items, StoredConversation{
			ID:        strconv.FormatUint(row.ID, 10),
			Type:      row.Type,
			Title:     row.Title,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return items, nil
}

// ListMessages returns visible history for one conversation.
// ListMessages 返回指定会话中调用方可见的历史消息。
func (s *GormStore) ListMessages(ctx context.Context, userID, conversationID string, beforeSequence uint64, limit int) ([]StoredMessage, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("message store is not initialized")
	}
	uid, err := parseUintID(userID, "user_id")
	if err != nil {
		return nil, err
	}
	convID, err := parseUintID(conversationID, "conversation_id")
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}

	var member model.ConversationMember
	err = s.db.WithContext(ctx).
		Where("conversation_id = ? AND user_id = ?", convID, uid).
		First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrConversationForbidden
	}
	if err != nil {
		return nil, fmt.Errorf("check conversation membership: %w", err)
	}

	query := s.db.WithContext(ctx).
		Where("conversation_id = ?", convID).
		Order("sequence DESC").
		Limit(limit)
	if beforeSequence > 0 {
		query = query.Where("sequence < ?", beforeSequence)
	}

	var rows []model.Message
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	items := make([]StoredMessage, 0, len(rows))
	for _, row := range rows {
		items = append(items, toStoredMessage(row))
	}
	return items, nil
}

// MarkDelivered records a successful recipient delivery.
// MarkDelivered 记录接收人投递成功。
func (s *GormStore) MarkDelivered(ctx context.Context, messageID, recipientUserID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("message store is not initialized")
	}
	recipientID, err := parseUintID(recipientUserID, "recipient_user_id")
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	err = s.db.WithContext(ctx).Model(&model.MessageDelivery{}).
		Where("message_id = ? AND recipient_user_id = ?", messageID, recipientID).
		Updates(map[string]any{
			"status":        model.DeliveryStatusDelivered,
			"delivered_at":  now,
			"last_error":    nil,
			"attempt_count": gorm.Expr("attempt_count + 1"),
		}).Error
	if err != nil {
		return fmt.Errorf("mark delivery delivered: %w", err)
	}
	return nil
}

// MarkDeliveryFailed records a failed recipient delivery attempt.
// MarkDeliveryFailed 记录接收人投递失败尝试。
func (s *GormStore) MarkDeliveryFailed(ctx context.Context, messageID, recipientUserID, reason string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("message store is not initialized")
	}
	recipientID, err := parseUintID(recipientUserID, "recipient_user_id")
	if err != nil {
		return err
	}
	err = s.db.WithContext(ctx).Model(&model.MessageDelivery{}).
		Where("message_id = ? AND recipient_user_id = ?", messageID, recipientID).
		Updates(map[string]any{
			"status":        model.DeliveryStatusFailed,
			"last_error":    reason,
			"attempt_count": gorm.Expr("attempt_count + 1"),
		}).Error
	if err != nil {
		return fmt.Errorf("mark delivery failed: %w", err)
	}
	return nil
}

func nextConversationSequence(ctx context.Context, tx *gorm.DB, conversationID uint64) (uint64, error) {
	var last model.Message
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("conversation_id = ?", conversationID).
		Order("sequence DESC").
		First(&last).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get next message sequence: %w", err)
	}
	return last.Sequence + 1, nil
}

func (s *GormStore) listRecipientUserIDs(ctx context.Context, messageID string) ([]string, error) {
	var deliveries []model.MessageDelivery
	if err := s.db.WithContext(ctx).
		Where("message_id = ?", messageID).
		Order("recipient_user_id ASC").
		Find(&deliveries).Error; err != nil {
		return nil, fmt.Errorf("list delivery recipients: %w", err)
	}
	recipients := make([]string, 0, len(deliveries))
	for _, delivery := range deliveries {
		recipients = append(recipients, strconv.FormatUint(delivery.RecipientUserID, 10))
	}
	return recipients, nil
}

func parseUintID(raw, name string) (uint64, error) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return id, nil
}

func toStoredMessage(row model.Message) StoredMessage {
	return StoredMessage{
		MessageID:       row.MessageID,
		ClientMessageID: row.ClientMessageID,
		ConversationID:  strconv.FormatUint(row.ConversationID, 10),
		FromUserID:      strconv.FormatUint(row.SenderUserID, 10),
		Content:         row.Content,
		Sequence:        row.Sequence,
		SentAt:          row.SentAt,
	}
}
