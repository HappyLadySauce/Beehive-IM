package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	defaultURL                 = "amqp://guest:guest@127.0.0.1:5672/"
	defaultExchange            = "beehive.im.push"
	defaultConnectTimeoutSecs  = 5
	defaultPublishTimeoutSecs  = 5
	defaultReconnectBackoffSec = 3
)

// Config describes RabbitMQ connection and topology settings.
// Config 描述 RabbitMQ 连接与拓扑配置。
type Config struct {
	URL                    string `json:",optional"`
	Exchange               string `json:",default=beehive.im.push"`
	ConnectTimeoutSeconds  int64  `json:",default=5"`
	PublishTimeoutSeconds  int64  `json:",default=5"`
	ReconnectBackoffSecond int64  `json:",default=3"`
	Prefetch               int    `json:",default=64"`
	PreferEnvWhenEmpty     bool   `json:",default=true"`
	DisableDefaultLocalURL bool   `json:",optional"`
}

// Normalize applies defaults and RABBITMQ_URL fallback.
// Normalize 应用默认值和 RABBITMQ_URL 回退。
func (c Config) Normalize() Config {
	if c.PreferEnvWhenEmpty || c.URL == "" {
		if env := strings.TrimSpace(os.Getenv("RABBITMQ_URL")); c.URL == "" && env != "" {
			c.URL = env
		}
	}
	if c.URL == "" && !c.DisableDefaultLocalURL {
		c.URL = defaultURL
	}
	if c.Exchange == "" {
		c.Exchange = defaultExchange
	}
	if c.ConnectTimeoutSeconds <= 0 {
		c.ConnectTimeoutSeconds = defaultConnectTimeoutSecs
	}
	if c.PublishTimeoutSeconds <= 0 {
		c.PublishTimeoutSeconds = defaultPublishTimeoutSecs
	}
	if c.ReconnectBackoffSecond <= 0 {
		c.ReconnectBackoffSecond = defaultReconnectBackoffSec
	}
	if c.Prefetch <= 0 {
		c.Prefetch = 64
	}
	return c
}

// Dial opens a RabbitMQ connection.
// Dial 打开 RabbitMQ 连接。
func Dial(c Config) (*amqp.Connection, error) {
	normalized := c.Normalize()
	if strings.TrimSpace(normalized.URL) == "" {
		return nil, errors.New("rabbitmq url is required")
	}
	conn, err := amqp.DialConfig(normalized.URL, amqp.Config{
		Dial: amqp.DefaultDial(time.Duration(normalized.ConnectTimeoutSeconds) * time.Second),
	})
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}
	return conn, nil
}

// DeclarePushTopology declares the exchange and one edge queue.
// DeclarePushTopology 声明 exchange 和一个 Edge 队列。
func DeclarePushTopology(ch *amqp.Channel, exchange, queue string) error {
	if ch == nil {
		return errors.New("rabbitmq channel is nil")
	}
	if exchange == "" {
		exchange = defaultExchange
	}
	if queue == "" {
		return errors.New("rabbitmq queue is required")
	}
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare rabbitmq exchange: %w", err)
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare rabbitmq queue: %w", err)
	}
	if err := ch.QueueBind(queue, queue, exchange, false, nil); err != nil {
		return fmt.Errorf("bind rabbitmq queue: %w", err)
	}
	return nil
}

// Publish sends one persistent JSON payload.
// Publish 发送一条持久化 JSON 消息。
func Publish(ctx context.Context, ch *amqp.Channel, exchange, routingKey string, body []byte) error {
	if ch == nil {
		return errors.New("rabbitmq channel is nil")
	}
	if exchange == "" {
		exchange = defaultExchange
	}
	if routingKey == "" {
		return errors.New("rabbitmq routing key is required")
	}
	err := ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
	if err != nil {
		return fmt.Errorf("publish rabbitmq message: %w", err)
	}
	return nil
}
