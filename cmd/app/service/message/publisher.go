package message

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"k8s.io/klog/v2"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/model"
)

const deliveryPublishCooldown = 2 * time.Second

// StartDeliveryPublisher starts bounded workers that publish pending deliveries to instance queues.
// StartDeliveryPublisher 启动有限 worker，将待投递记录发布到实例队列。
func (s *MessageService) StartDeliveryPublisher(ctx context.Context, workers int) error {
	if s == nil || s.DB == nil || s.MQ == nil || s.Presence == nil {
		return fmt.Errorf("message publisher is not configured")
	}
	if workers <= 0 {
		return fmt.Errorf("publisher workers must be > 0")
	}

	klog.InfoS("starting message delivery publisher", "workers", workers, "batchSize", s.PublishBatchSize)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			s.runDeliveryPublisher(ctx, workerID)
		}(i)
	}

	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

func (s *MessageService) runDeliveryPublisher(ctx context.Context, workerID int) {
	ticker := time.NewTicker(s.PublishScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.publishPendingBatch(ctx); err != nil && ctx.Err() == nil {
				klog.ErrorS(err, "publish pending deliveries", "workerID", workerID)
			}
		}
	}
}

func (s *MessageService) publishPendingBatch(ctx context.Context) error {
	deliveries, err := s.claimPublishBatch(ctx)
	if err != nil {
		return err
	}
	for _, delivery := range deliveries {
		if err := s.publishOneDelivery(ctx, delivery); err != nil {
			klog.ErrorS(err, "publish delivery", "deliveryID", delivery.ID, "messageID", delivery.MessageID, "recipientUserID", delivery.RecipientUserID)
		}
	}
	return nil
}

func (s *MessageService) claimPublishBatch(ctx context.Context) ([]model.MessageDelivery, error) {
	var deliveries []model.MessageDelivery
	cutoff := time.Now().UTC().Add(-deliveryPublishCooldown)
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(
				"status IN ? AND attempt_count < ? AND updated_at <= ?",
				[]string{model.DeliveryStatusPending, model.DeliveryStatusFailed},
				s.DeliveryMaxAttempts,
				cutoff,
			).
			Order("updated_at ASC").
			Limit(s.PublishBatchSize).
			Find(&deliveries).Error; err != nil {
			return err
		}
		if len(deliveries) == 0 {
			return nil
		}
		ids := make([]uint64, 0, len(deliveries))
		for _, delivery := range deliveries {
			ids = append(ids, delivery.ID)
		}
		return tx.Model(&model.MessageDelivery{}).
			Where("id IN ?", ids).
			Update("updated_at", time.Now().UTC()).Error
	})
	if err != nil {
		return nil, fmt.Errorf("claim publish batch: %w", err)
	}
	return deliveries, nil
}

func (s *MessageService) publishOneDelivery(ctx context.Context, delivery model.MessageDelivery) error {
	instances, err := s.Presence.InstancesForUser(ctx, FormatUserID(delivery.RecipientUserID))
	if err != nil {
		_ = s.markDeliveryPublishFailure(ctx, delivery, err)
		return err
	}
	if len(instances) == 0 {
		_ = s.touchDelivery(ctx, delivery.ID, "recipient offline")
		return nil
	}

	var stored model.Message
	if err := s.DB.WithContext(ctx).Where("message_id = ?", delivery.MessageID).First(&stored).Error; err != nil {
		err = fmt.Errorf("load message for delivery: %w", err)
		_ = s.markDeliveryPublishFailure(ctx, delivery, err)
		return err
	}

	body := DeliverPayloadFromModel(stored, stored.SenderUserID, delivery.RecipientUserID)
	payload, err := json.Marshal(body)
	if err != nil {
		_ = s.markDeliveryPublishFailure(ctx, delivery, err)
		return fmt.Errorf("marshal deliver payload: %w", err)
	}

	for _, instanceID := range instances {
		if err := s.MQ.SendMessageWithConfirm(ctx, DeliverInstanceTopic(instanceID), payload, s.PublishTimeout); err != nil {
			_ = s.markDeliveryPublishFailure(ctx, delivery, err)
			return fmt.Errorf("publish deliver event to instance %q: %w", instanceID, err)
		}
	}
	return s.markDeliveryPublished(ctx, delivery.ID)
}

func (s *MessageService) markDeliveryPublished(ctx context.Context, deliveryID uint64) error {
	now := time.Now().UTC()
	result := s.DB.WithContext(ctx).
		Model(&model.MessageDelivery{}).
		Where("id = ? AND status IN ?", deliveryID, []string{model.DeliveryStatusPending, model.DeliveryStatusFailed}).
		Updates(map[string]any{
			"status":       model.DeliveryStatusPublished,
			"published_at": now,
			"last_error":   nil,
		})
	if result.Error != nil {
		return fmt.Errorf("mark delivery published: %w", result.Error)
	}
	return nil
}

func (s *MessageService) markDeliveryPublishFailure(ctx context.Context, delivery model.MessageDelivery, cause error) error {
	attempts := delivery.AttemptCount + 1
	status := model.DeliveryStatusPending
	if attempts >= s.DeliveryMaxAttempts {
		status = model.DeliveryStatusFailed
	}
	msg := cause.Error()
	result := s.DB.WithContext(ctx).
		Model(&model.MessageDelivery{}).
		Where("id = ?", delivery.ID).
		Updates(map[string]any{
			"status":        status,
			"attempt_count": attempts,
			"last_error":    msg,
		})
	if result.Error != nil {
		return fmt.Errorf("mark delivery publish failure: %w", result.Error)
	}
	return nil
}

func (s *MessageService) touchDelivery(ctx context.Context, deliveryID uint64, reason string) error {
	result := s.DB.WithContext(ctx).
		Model(&model.MessageDelivery{}).
		Where("id = ? AND status = ?", deliveryID, model.DeliveryStatusPending).
		Updates(map[string]any{
			"last_error": reason,
		})
	if result.Error != nil {
		return fmt.Errorf("touch delivery: %w", result.Error)
	}
	return nil
}

// MarkDeliveryDispatched records that this instance consumed the delivery event.
// MarkDeliveryDispatched 记录当前实例已消费该投递事件。
func (s *MessageService) MarkDeliveryDispatched(ctx context.Context, messageID string, recipientUserID uint64) error {
	now := time.Now().UTC()
	result := s.DB.WithContext(ctx).
		Model(&model.MessageDelivery{}).
		Where(
			"message_id = ? AND recipient_user_id = ? AND status IN ?",
			messageID,
			recipientUserID,
			[]string{model.DeliveryStatusPublished, model.DeliveryStatusDispatched},
		).
		Updates(map[string]any{
			"status":        model.DeliveryStatusDispatched,
			"dispatched_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("mark delivery dispatched: %w", result.Error)
	}
	return nil
}

// MarkDeliveryDelivered records successful enqueue to at least one websocket session.
// MarkDeliveryDelivered 记录消息已成功进入至少一个 WebSocket 会话发送队列。
func (s *MessageService) MarkDeliveryDelivered(ctx context.Context, messageID string, recipientUserID uint64) error {
	now := time.Now().UTC()
	result := s.DB.WithContext(ctx).
		Model(&model.MessageDelivery{}).
		Where(
			"message_id = ? AND recipient_user_id = ? AND status IN ?",
			messageID,
			recipientUserID,
			[]string{model.DeliveryStatusPublished, model.DeliveryStatusDispatched},
		).
		Updates(map[string]any{
			"status":       model.DeliveryStatusDelivered,
			"delivered_at": now,
			"last_error":   nil,
		})
	if result.Error != nil {
		return fmt.Errorf("mark delivery delivered: %w", result.Error)
	}
	return nil
}
