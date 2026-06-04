package mq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// SendMessage publishes a persistent JSON message to the configured topic exchange.
// SendMessage 向已配置的 Topic 交换机发布持久化 JSON 消息。
func (c *Client) SendMessage(ctx context.Context, topic string, message []byte) error {
	if c == nil {
		return fmt.Errorf("mq client is nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channel == nil {
		return fmt.Errorf("publish channel is nil")
	}

	err := c.channel.PublishWithContext(ctx, c.exchange, topic, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         message,
	})
	if err != nil {
		return fmt.Errorf("publish message: %w", err)
	}
	return nil
}
