package mq

import (
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

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.channel != nil {
		return c.channel.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}