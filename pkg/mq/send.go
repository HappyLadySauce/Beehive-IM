package mq

import (
	"fmt"
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

// SendMessage sends a message to the specified exchange and topic.
// SendMessage 发送消息到指定的交换机和主题。
func (c *Client) SendMessage(ctx context.Context, topic string, message []byte) error {
	if c == nil {
		return fmt.Errorf("mq client is nil")
	}
	if c.channel == nil {
		c.mu.RLock()
		channel, err := c.conn.Channel()
		if err != nil {
			c.mu.RUnlock()
			return fmt.Errorf("failed to open a channel: %w", err)
		}
		c.channel = channel
		c.mu.RUnlock()
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	err := c.channel.PublishWithContext(ctx, c.exchange, topic, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        message,
	})
	if err != nil {
		return fmt.Errorf("failed to publish message: %v", err)
	}

	return nil
}
