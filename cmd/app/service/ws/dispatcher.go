package ws

import (
	"context"
	"fmt"
	"time"

	msgsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/message"
	"k8s.io/klog/v2"
)

// MessageDispatcher consumes durable message events and dispatches them to local websocket clients.
// MessageDispatcher 消费可靠消息事件并投递到本机 WebSocket 客户端。
type MessageDispatcher struct {
	store msgsvc.Store
	hub   *Hub
}

func NewMessageDispatcher(store msgsvc.Store, hub *Hub) *MessageDispatcher {
	return &MessageDispatcher{store: store, hub: hub}
}

// HandleMessageCreated dispatches one message-created event to all local online recipients.
// HandleMessageCreated 将单条消息创建事件投递给所有本机在线接收人。
func (d *MessageDispatcher) HandleMessageCreated(ctx context.Context, event msgsvc.MessageCreatedEvent) error {
	if d == nil || d.store == nil || d.hub == nil {
		return fmt.Errorf("message dispatcher is not initialized")
	}
	if event.MessageID == "" {
		return fmt.Errorf("message_id is required")
	}

	for _, recipientID := range event.RecipientUserIDs {
		payload := MessageReceivePayload{
			MessageID:      event.MessageID,
			ConversationID: event.ConversationID,
			FromUserID:     event.FromUserID,
			ToUserID:       recipientID,
			Content:        event.Content,
			Sequence:       event.Sequence,
			SentAt:         event.SentAt,
		}
		envelope, err := newEnvelope(event.MessageID, TypeMessageReceive, payload)
		if err != nil {
			return fmt.Errorf("encode websocket receive envelope: %w", err)
		}
		if event.SentAt > 0 {
			envelope.Timestamp = event.SentAt
		} else {
			envelope.Timestamp = time.Now().UnixMilli()
		}

		if d.hub.DeliverToOnlineUser(recipientID, envelope) {
			if err := d.store.MarkDelivered(ctx, event.MessageID, recipientID); err != nil {
				return err
			}
			continue
		}
		if err := d.store.MarkDeliveryFailed(ctx, event.MessageID, recipientID, "recipient is not connected to this node"); err != nil {
			return err
		}
		klog.InfoS("Message recipient is offline on this node", "messageID", event.MessageID, "recipientUserID", recipientID)
	}
	return nil
}
