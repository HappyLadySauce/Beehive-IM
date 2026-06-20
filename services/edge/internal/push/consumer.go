package push

import (
	"context"
	"encoding/json"
	"os"
	"time"

	pkgrabbitmq "github.com/HappyLadySauce/Beehive-IM/pkg/rabbitmq"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/wsproxy"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"
)

// Message is the minimal Edge push payload consumed from RabbitMQ.
// Message 是从 RabbitMQ 消费的最小 Edge push 载荷。
type Message struct {
	ConnID    string          `json:"conn_id"`
	SessionID string          `json:"session_id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// Consumer consumes edge.push.{edge_id} and writes to local WebSocket queues.
// Consumer 消费 edge.push.{edge_id} 并写入本机 WebSocket 队列。
type Consumer struct {
	config pkgrabbitmq.Config
	edgeID string
	proxy  *wsproxy.Proxy
	stop   chan struct{}
}

func NewConsumer(edgeID string, config pkgrabbitmq.Config, proxy *wsproxy.Proxy) *Consumer {
	return &Consumer{
		config: config,
		edgeID: edgeID,
		proxy:  proxy,
		stop:   make(chan struct{}),
	}
}

func (c *Consumer) Start(ctx context.Context) {
	if c == nil || c.proxy == nil {
		return
	}
	if c.config.URL == "" && os.Getenv("RABBITMQ_URL") == "" {
		logx.Info("edge push consumer disabled: RabbitMQ URL is empty")
		return
	}
	go c.run(ctx)
}

func (c *Consumer) Stop() {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
}

func (c *Consumer) run(ctx context.Context) {
	cfg := c.config.Normalize()
	queue := "edge.push." + c.edgeID
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stop:
			return
		default:
		}

		if err := c.consumeOnce(ctx, cfg, queue); err != nil {
			logx.Errorf("edge push consumer stopped: %v", err)
		}

		select {
		case <-time.After(time.Duration(cfg.ReconnectBackoffSecond) * time.Second):
		case <-ctx.Done():
			return
		case <-c.stop:
			return
		}
	}
}

func (c *Consumer) consumeOnce(ctx context.Context, cfg pkgrabbitmq.Config, queue string) error {
	conn, err := pkgrabbitmq.Dial(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := pkgrabbitmq.DeclarePushTopology(ch, cfg.Exchange, queue); err != nil {
		return err
	}
	if err := ch.Qos(cfg.Prefetch, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stop:
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return amqp.ErrClosed
			}
			c.handleDelivery(ctx, delivery)
		}
	}
}

func (c *Consumer) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	var msg Message
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		logx.Errorf("edge push decode failed: %v", err)
		ack(delivery)
		return
	}
	frame, err := FramePayload(msg)
	if err != nil {
		logx.Errorf("edge push encode failed: %v", err)
		ack(delivery)
		return
	}
	delivered := c.proxy.Deliver(ctx, wsproxy.PushTarget{ConnID: msg.ConnID, SessionID: msg.SessionID}, frame)
	if !delivered {
		logx.Infof("edge push dropped: conn_id=%s session_id=%s", msg.ConnID, msg.SessionID)
	}
	ack(delivery)
}

func FramePayload(msg Message) ([]byte, error) {
	frameType := msg.Type
	if frameType == "" {
		frameType = "push"
	}
	payload := json.RawMessage(msg.Payload)
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	return json.Marshal(map[string]any{
		"type":    frameType,
		"payload": payload,
	})
}

func ack(delivery amqp.Delivery) {
	if delivery.Acknowledger != nil {
		_ = delivery.Ack(false)
	}
}
