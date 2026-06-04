package ws

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"k8s.io/klog/v2"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/service/message"
	"github.com/HappyLadySauce/Beehive-IM/pkg/mq"
)

// StartDeliveryConsumer binds the dispatch queue and pushes MQ events to connected clients.
// StartDeliveryConsumer 绑定分发队列并将 MQ 事件推送给在线客户端。
func (h *Hub) StartDeliveryConsumer(ctx context.Context, mqClient *mq.Client, queueName string, prefetch int) error {
	if h == nil {
		return fmt.Errorf("hub is nil")
	}
	if mqClient == nil {
		return fmt.Errorf("mq client is nil")
	}
	if err := mqClient.EnsureDispatchQueue(queueName, message.DeliverTopicPattern); err != nil {
		return err
	}

	klog.InfoS("starting message delivery consumer", "queue", queueName, "exchange", mqClient.Exchange())

	return mqClient.Consume(ctx, queueName, prefetch, func(ctx context.Context, delivery amqp.Delivery) error {
		var payload message.MessageDeliverPayload
		if err := json.Unmarshal(delivery.Body, &payload); err != nil {
			return fmt.Errorf("unmarshal deliver payload: %w", err)
		}
		return h.DeliverToUser(ctx, payload)
	})
}
