package mq

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"

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
	return c.ConsumeSharded(ctx, queueName, prefetch, 1, nil, handler)
}

// ConsumeSharded consumes a queue with bounded workers and routes equal shard keys to the same worker.
// ConsumeSharded 使用有限 worker 消费队列，并将相同分片键路由到同一个 worker。
func (c *Client) ConsumeSharded(
	ctx context.Context,
	queueName string,
	prefetch int,
	workers int,
	shardKey func(amqp.Delivery) string,
	handler func(context.Context, amqp.Delivery) error,
) error {
	if c == nil {
		return fmt.Errorf("mq client is nil")
	}
	if handler == nil {
		return fmt.Errorf("handler is nil")
	}

	if workers <= 0 {
		return fmt.Errorf("workers must be > 0")
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

	jobs := make([]chan amqp.Delivery, workers)
	results := make(chan consumeResult, prefetch+workers)
	var wg sync.WaitGroup
	for i := range jobs {
		jobs[i] = make(chan amqp.Delivery, prefetch)
		wg.Add(1)
		go func(jobCh <-chan amqp.Delivery) {
			defer wg.Done()
			for delivery := range jobCh {
				result := consumeResult{
					delivery: delivery,
					err:      handler(ctx, delivery),
				}
				select {
				case results <- result:
				case <-ctx.Done():
					return
				}
			}
		}(jobs[i])
	}
	defer func() {
		for _, jobCh := range jobs {
			close(jobCh)
		}
		wg.Wait()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("delivery channel closed")
			}
			workerIndex := 0
			if shardKey != nil && workers > 1 {
				workerIndex = shardIndex(shardKey(delivery), workers)
			}
			select {
			case jobs[workerIndex] <- delivery:
			case result := <-results:
				if result.err != nil {
					_ = result.delivery.Nack(false, true)
					continue
				}
				_ = result.delivery.Ack(false)
				jobs[workerIndex] <- delivery
			case <-ctx.Done():
				return ctx.Err()
			}
		case result := <-results:
			if result.err != nil {
				_ = result.delivery.Nack(false, true)
				continue
			}
			_ = result.delivery.Ack(false)
		}
	}
}

type consumeResult struct {
	delivery amqp.Delivery
	err      error
}

func shardIndex(key string, workers int) int {
	if key == "" || workers <= 1 {
		return 0
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	return int(hash.Sum32() % uint32(workers))
}
