package mq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn    *amqp.Connection
}

func NewClient(url string) (*Client, error) {
	// Open a connection to the RabbitMQ server.
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	return &Client{conn: conn}, nil
}

func (c *Client) Channel() (*amqp.Channel, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %v", err)
	}
	return ch, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}