package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"k8s.io/klog/v2"

	msgsvc "github.com/HappyLadySauce/Beehive-IM/cmd/app/service/message"
	"github.com/HappyLadySauce/Beehive-IM/pkg/options"
)

// Client owns the RabbitMQ connection, publishing channel, and topology metadata.
// Client 持有 RabbitMQ 连接、发布通道与拓扑元数据。
type Client struct {
	cfg     *options.RabbitMQOptions
	conn    *amqp.Connection
	channel *amqp.Channel
	mu      sync.Mutex
}

// NewClient connects to RabbitMQ and declares the IM event topology.
// NewClient 连接 RabbitMQ 并声明 IM 事件拓扑。
func NewClient(ctx context.Context, cfg *options.RabbitMQOptions) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("rabbitmq config is nil")
	}
	conn, err := amqp.Dial(amqpURL(cfg))
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("enable rabbitmq publisher confirms: %w", err)
	}

	client := &Client{cfg: cfg, conn: conn, channel: ch}
	if err := client.declareTopology(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	klog.InfoS("RabbitMQ connection established", "exchange", cfg.Exchange, "queue", cfg.Queue)
	return client, nil
}

// PublishMessageCreated publishes a confirmed message-created event.
// PublishMessageCreated 发布带确认的消息创建事件。
func (c *Client) PublishMessageCreated(ctx context.Context, event msgsvc.MessageCreatedEvent) error {
	return c.publishJSON(ctx, msgsvc.EventRoutingKeyMessageCreated, event)
}

// StartMessageConsumer starts concurrent workers for message-created events.
// StartMessageConsumer 启动并发 worker 消费消息创建事件。
func (c *Client) StartMessageConsumer(ctx context.Context, handler func(context.Context, msgsvc.MessageCreatedEvent) error) error {
	if c == nil || c.channel == nil {
		return fmt.Errorf("rabbitmq client is not initialized")
	}
	if handler == nil {
		return fmt.Errorf("message handler is nil")
	}
	if err := c.channel.Qos(c.cfg.Prefetch, 0, false); err != nil {
		return fmt.Errorf("set rabbitmq qos: %w", err)
	}
	deliveries, err := c.channel.ConsumeWithContext(
		ctx,
		c.cfg.Queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("start rabbitmq consumer: %w", err)
	}

	for i := 0; i < c.cfg.ConsumeConcurrency; i++ {
		go c.consumeMessages(ctx, i, deliveries, handler)
	}
	return nil
}

// Close releases RabbitMQ resources.
// Close 释放 RabbitMQ 资源。
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var err error
	if c.channel != nil {
		err = c.channel.Close()
	}
	if c.conn != nil {
		if closeErr := c.conn.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

func (c *Client) declareTopology(ctx context.Context) error {
	if c == nil || c.channel == nil {
		return fmt.Errorf("rabbitmq client is not initialized")
	}
	if err := c.channel.ExchangeDeclare(
		c.cfg.Exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare rabbitmq exchange: %w", err)
	}
	if _, err := c.channel.QueueDeclare(
		c.cfg.Queue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare rabbitmq queue: %w", err)
	}
	if err := c.channel.QueueBind(
		c.cfg.Queue,
		msgsvc.EventRoutingKeyMessageCreated,
		c.cfg.Exchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind rabbitmq queue: %w", err)
	}
	return ctx.Err()
}

func (c *Client) publishJSON(ctx context.Context, routingKey string, payload any) error {
	if c == nil || c.channel == nil {
		return fmt.Errorf("rabbitmq client is not initialized")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode rabbitmq payload: %w", err)
	}
	publishCtx, cancel := context.WithTimeout(ctx, c.cfg.PublishTimeout)
	defer cancel()

	c.mu.Lock()
	defer c.mu.Unlock()
	confirmation, err := c.channel.PublishWithDeferredConfirmWithContext(
		publishCtx,
		c.cfg.Exchange,
		routingKey,
		true,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish rabbitmq message: %w", err)
	}
	if confirmation == nil {
		return fmt.Errorf("rabbitmq publish confirmation is nil")
	}
	confirmed, err := confirmation.WaitContext(publishCtx)
	if err != nil {
		return fmt.Errorf("wait rabbitmq publish confirmation: %w", err)
	}
	if !confirmed {
		return fmt.Errorf("rabbitmq publish was not confirmed")
	}
	return nil
}

func (c *Client) consumeMessages(ctx context.Context, workerID int, deliveries <-chan amqp.Delivery, handler func(context.Context, msgsvc.MessageCreatedEvent) error) {
	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-deliveries:
			if !ok {
				return
			}
			var event msgsvc.MessageCreatedEvent
			if err := json.Unmarshal(delivery.Body, &event); err != nil {
				klog.ErrorS(err, "failed to decode rabbitmq message", "workerID", workerID, "routingKey", delivery.RoutingKey)
				_ = delivery.Nack(false, false)
				continue
			}
			if err := handler(ctx, event); err != nil {
				klog.ErrorS(err, "failed to handle rabbitmq message", "workerID", workerID, "messageID", event.MessageID)
				_ = delivery.Nack(false, true)
				continue
			}
			if err := delivery.Ack(false); err != nil {
				klog.ErrorS(err, "failed to ack rabbitmq message", "workerID", workerID, "messageID", event.MessageID)
			}
		}
	}
}

func amqpURL(cfg *options.RabbitMQOptions) string {
	u := &url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:   cfg.VirtualHost,
	}
	if cfg.VirtualHost == "/" {
		u.Path = "/"
	}
	return u.String()
}
