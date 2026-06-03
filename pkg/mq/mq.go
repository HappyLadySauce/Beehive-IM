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


func (c *Client) Close() error {
	return c.conn.Close()
}