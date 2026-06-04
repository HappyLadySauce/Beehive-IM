package mq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// EnsureDispatchQueue declares a durable queue and binds it to the delivery routing pattern.
// EnsureDispatchQueue 声明持久化队列并绑定到投递路由模式。
func (c *Client) EnsureDispatchQueue(queueName, bindingPattern string) error {
	if c == nil {
		return fmt.Errorf("mq client is nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channel == nil {
		return fmt.Errorf("channel is nil")
	}
	if _, err := c.channel.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare queue %q: %w", queueName, err)
	}
	if err := c.channel.QueueBind(queueName, bindingPattern, c.exchange, false, nil); err != nil {
		return fmt.Errorf("bind queue %q: %w", queueName, err)
	}
	return nil
}

// Consume starts a blocking consume loop until ctx is cancelled.
// Consume 启动消费循环，直到 ctx 被取消。
func (c *Client) Consume(ctx context.Context, queueName string, prefetch int, handler func(context.Context, amqp.Delivery) error) error {
	if c == nil {
		return fmt.Errorf("mq client is nil")
	}
	if handler == nil {
		return fmt.Errorf("handler is nil")
	}

	consumeCh, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("open consume channel: %w", err)
	}
	defer consumeCh.Close()

	if err := consumeCh.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	deliveries, err := consumeCh.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume queue %q: %w", queueName, err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("delivery channel closed")
			}
			if err := handler(ctx, delivery); err != nil {
				_ = delivery.Nack(false, true)
				continue
			}
			_ = delivery.Ack(false)
		}
	}
}
