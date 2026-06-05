package message

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/model"
)

// SendMessage validates membership, persists the message, and creates async delivery rows.
// SendMessage 校验成员关系、落库，并创建异步投递记录。
func (s *MessageService) SendMessage(ctx context.Context, senderUserID uint64, req MessageSendPayload) (*MessageSendResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("message service is not configured")
	}

	params, err := req.Parse()
	if err != nil {
		return nil, err
	}

	var (
		stored       model.Message
		recipientIDs []uint64
	)

	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureConversationExists(tx, params.ConversationID); err != nil {
			return err
		}
		if err := ensureSenderMember(tx, params.ConversationID, senderUserID); err != nil {
			return err
		}

		hit, err := tryLoadIdempotentMessage(tx, params, senderUserID, &stored, &recipientIDs)
		if err != nil {
			return err
		}
		if hit {
			return nil
		}

		if _, err := createMessageWithDeliveries(tx, params, senderUserID, &stored, &recipientIDs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return ResultFromModel(stored), nil
}

func ensureConversationExists(tx *gorm.DB, convID uint64) error {
	var convCount int64
	if err := tx.Model(&model.Conversation{}).
		Where("id = ?", convID).
		Count(&convCount).Error; err != nil {
		return fmt.Errorf("check conversation existence: %w", err)
	}
	if convCount == 0 {
		return ErrConversationNotFound
	}
	return nil
}

func ensureSenderMember(tx *gorm.DB, convID, senderUserID uint64) error {
	var memberCount int64
	if err := tx.Model(&model.ConversationMember{}).
		Where("conversation_id = ? AND user_id = ?", convID, senderUserID).
		Count(&memberCount).Error; err != nil {
		return fmt.Errorf("check conversation membership: %w", err)
	}
	if memberCount == 0 {
		return ErrNotConversationMember
	}
	return nil
}

func lockConversationForUpdate(tx *gorm.DB, convID uint64) error {
	var conv model.Conversation
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", convID).
		First(&conv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrConversationNotFound
	}
	if err != nil {
		return fmt.Errorf("lock conversation: %w", err)
	}
	return nil
}

func loadPendingRecipientIDs(tx *gorm.DB, messageID string, recipientIDs *[]uint64) error {
	*recipientIDs = (*recipientIDs)[:0]
	return tx.Model(&model.MessageDelivery{}).
		Where("message_id = ? AND status = ?", messageID, model.DeliveryStatusPending).
		Pluck("recipient_user_id", recipientIDs).Error
}

func tryLoadIdempotentMessage(
	tx *gorm.DB,
	params SendParams,
	senderUserID uint64,
	stored *model.Message,
	recipientIDs *[]uint64,
) (bool, error) {
	var existing model.Message
	findErr := tx.Where("sender_user_id = ? AND client_message_id = ?", senderUserID, params.ClientMessageID).
		First(&existing).Error
	if errors.Is(findErr, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if findErr != nil {
		return false, fmt.Errorf("lookup idempotent message: %w", findErr)
	}
	if existing.ConversationID != params.ConversationID {
		return false, ErrConversationMismatch
	}
	*stored = existing
	if err := loadPendingRecipientIDs(tx, stored.MessageID, recipientIDs); err != nil {
		return false, fmt.Errorf("list pending deliveries: %w", err)
	}
	return true, nil
}

// createMessageWithDeliveries allocates sequence under a conversation row lock and creates delivery rows.
// createMessageWithDeliveries 在会话行锁下分配序号并创建投递记录。
// The bool result is retained for callers that distinguish insert vs idempotent reload; SendMessage ignores it.
// bool 返回值供需要区分「新建」与「幂等重载」的调用方使用；SendMessage 当前不依赖该值。
func createMessageWithDeliveries(
	tx *gorm.DB,
	params SendParams,
	senderUserID uint64,
	stored *model.Message,
	recipientIDs *[]uint64,
) (bool, error) {
	if err := lockConversationForUpdate(tx, params.ConversationID); err != nil {
		return false, err
	}

	var maxSeq uint64
	if err := tx.Model(&model.Message{}).
		Where("conversation_id = ?", params.ConversationID).
		Select("COALESCE(MAX(sequence), 0)").
		Scan(&maxSeq).Error; err != nil {
		return false, fmt.Errorf("allocate sequence: %w", err)
	}

	now := time.Now().UTC()
	*stored = model.Message{
		MessageID:       uuid.NewString(),
		ConversationID:  params.ConversationID,
		SenderUserID:    senderUserID,
		ClientMessageID: params.ClientMessageID,
		Content:         params.Content,
		Status:          model.MessageStatusCreated,
		Sequence:        maxSeq + 1,
		SentAt:          now,
	}
	if err := tx.Create(stored).Error; err != nil {
		if isPostgresUniqueViolation(err) {
			hit, loadErr := tryLoadIdempotentMessage(tx, params, senderUserID, stored, recipientIDs)
			if loadErr != nil {
				return false, loadErr
			}
			if hit {
				return false, nil
			}
		}
		return false, fmt.Errorf("create message: %w", err)
	}

	var members []model.ConversationMember
	if err := tx.Where("conversation_id = ?", params.ConversationID).Find(&members).Error; err != nil {
		return false, fmt.Errorf("list conversation members: %w", err)
	}
	for _, member := range members {
		if member.UserID == senderUserID {
			continue
		}
		*recipientIDs = append(*recipientIDs, member.UserID)
		delivery := model.MessageDelivery{
			MessageID:       stored.MessageID,
			ConversationID:  params.ConversationID,
			RecipientUserID: member.UserID,
			Status:          model.DeliveryStatusPending,
		}
		if err := tx.Create(&delivery).Error; err != nil {
			return false, fmt.Errorf("create message delivery: %w", err)
		}
	}
	return true, nil
}
