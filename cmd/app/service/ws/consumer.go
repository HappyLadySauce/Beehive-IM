package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
	"k8s.io/klog/v2"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/service/message"
	"github.com/HappyLadySauce/Beehive-IM/pkg/mq"
)

// StartDeliveryConsumer binds the dispatch queue and pushes MQ events to connected clients.
// StartDeliveryConsumer 绑定分发队列并将 MQ 事件推送给在线客户端。
func (h *Hub) StartDeliveryConsumer(ctx context.Context, mqClient *mq.Client, baseQueueName, instanceID string, prefetch, workers int) error {
	if h == nil {
		return fmt.Errorf("hub is nil")
	}
	if mqClient == nil {
		return fmt.Errorf("mq client is nil")
	}
	queueName := message.InstanceQueueName(baseQueueName, instanceID)
	bindingKey := message.DeliverInstanceTopic(instanceID)
	if err := mqClient.EnsureDispatchQueue(queueName, bindingKey); err != nil {
		return err
	}

	klog.InfoS("starting message delivery consumer", "queue", queueName, "exchange", mqClient.Exchange(), "workers", workers)

	return mqClient.ConsumeSharded(ctx, queueName, prefetch, workers, deliveryShardKey, func(ctx context.Context, delivery amqp.Delivery) error {
		var payload message.MessageDeliverPayload
		if err := json.Unmarshal(delivery.Body, &payload); err != nil {
			return fmt.Errorf("unmarshal deliver payload: %w", err)
		}
		return h.HandleDelivery(ctx, payload)
	})
}

// HandleDelivery records dispatch progress and enqueues the message to local websocket sessions.
// HandleDelivery 记录分发进度，并将消息写入本地 WebSocket 会话发送队列。
func (h *Hub) HandleDelivery(ctx context.Context, payload message.MessageDeliverPayload) error {
	if h == nil || h.messages == nil {
		return fmt.Errorf("hub message service is not configured")
	}
	recipientID, err := strconv.ParseUint(payload.RecipientUserID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid recipient user id: %w", err)
	}
	if err := h.messages.MarkDeliveryDispatched(ctx, payload.MessageID, recipientID); err != nil {
		return err
	}
	result, err := h.DeliverToUser(ctx, payload)
	if err != nil {
		return err
	}
	if result.Enqueued > 0 {
		return h.messages.MarkDeliveryDelivered(ctx, payload.MessageID, recipientID)
	}
	klog.InfoS("delivery dispatched without local websocket session",
		"messageID", payload.MessageID,
		"recipientUserID", payload.RecipientUserID,
		"onlineSessions", result.OnlineSessions,
		"failed", result.Failed,
	)
	return nil
}

func deliveryShardKey(delivery amqp.Delivery) string {
	var payload message.MessageDeliverPayload
	if err := json.Unmarshal(delivery.Body, &payload); err != nil {
		return ""
	}
	return payload.RecipientUserID
}
