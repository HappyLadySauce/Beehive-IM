package mq

import (
	"errors"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	exchange string
	mu	sync.RWMutex
}

func NewClient(url string, exchange string) (*Client, error) {
	// Open a connection to the RabbitMQ server.
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %v", err)
	}
	if err := channel.ExchangeDeclare(
		exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return nil, fmt.Errorf("failed to declare exchange: %v", err)
	}

	return &Client{conn: conn, channel: channel, exchange: exchange}, nil
}

// Exchange returns the configured topic exchange name.
// Exchange 返回已配置的 Topic 交换机名称。
func (c *Client) Exchange() string {
	if c == nil {
		return ""
	}
	return c.exchange
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	if c.channel != nil {
		err = c.channel.Close()
		c.channel = nil
	}
	if c.conn != nil {
		err = errors.Join(err, c.conn.Close())
		c.conn = nil
	}
	return err
}