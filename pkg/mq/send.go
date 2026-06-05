package mq

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// SendMessage publishes a persistent JSON message to the configured topic exchange.
// SendMessage 向已配置的 Topic 交换机发布持久化 JSON 消息。
func (c *Client) SendMessage(ctx context.Context, topic string, message []byte) error {
	return c.SendMessageWithConfirm(ctx, topic, message, 0)
}

// SendMessageWithConfirm publishes a persistent JSON message and waits for broker confirmation.
// SendMessageWithConfirm 发布持久化 JSON 消息并等待 broker 确认。
func (c *Client) SendMessageWithConfirm(ctx context.Context, topic string, message []byte, timeout time.Duration) error {
	if c == nil {
		return fmt.Errorf("mq client is nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channel == nil {
		return fmt.Errorf("publish channel is nil")
	}
	if c.confirms == nil {
		return fmt.Errorf("publish confirm channel is nil")
	}

	err := c.channel.PublishWithContext(ctx, c.exchange, topic, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         message,
	})
	if err != nil {
		return fmt.Errorf("publish message: %w", err)
	}
	waitCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	select {
	case <-waitCtx.Done():
		return fmt.Errorf("wait publish confirm: %w", waitCtx.Err())
	case confirm, ok := <-c.confirms:
		if !ok {
			return fmt.Errorf("publish confirm channel closed")
		}
		if !confirm.Ack {
			return fmt.Errorf("publish rejected by broker")
		}
	}
	return nil
}
