package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	pkgrabbitmq "github.com/HappyLadySauce/Beehive-IM/pkg/rabbitmq"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/conversationservice"
	"github.com/HappyLadySauce/Beehive-IM/services/notification/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/presenceservice"
	amqp "github.com/rabbitmq/amqp091-go"
	goredis "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	defaultQueue         = "notification.message.events"
	defaultBindingKey    = "message.created.#"
	defaultPushExchange  = "beehive.im.push"
	defaultWorkerCount   = 2
	defaultDedupeTTL     = 24 * time.Hour
	defaultPublishTimout = 5 * time.Second
)

// Config describes the notification event consumer.
// Config 描述通知事件消费者配置。
type Config struct {
	RabbitMQ       pkgrabbitmq.Config
	Queue          string
	BindingKey     string
	PushExchange   string
	WorkerCount    int
	DedupeTTL      time.Duration
	PublishTimeout time.Duration
}

// Dependencies contains RPC clients and stores used by the consumer.
// Dependencies 包含消费者使用的 RPC 客户端与存储。
type Dependencies struct {
	Redis        *goredis.Client
	Deliveries   *repository.Repository
	Conversation conversationservice.ConversationService
	Presence     presenceservice.PresenceService
}

// Consumer consumes message events and publishes online Edge push frames.
// Consumer 消费消息事件并发布在线 Edge 推送帧。
type Consumer struct {
	config Config
	deps   Dependencies
	stop   chan struct{}
}

func NewConsumer(config Config, deps Dependencies) *Consumer {
	return &Consumer{
		config: normalizeConfig(config),
		deps:   deps,
		stop:   make(chan struct{}),
	}
}

func (c *Consumer) Start(ctx context.Context) {
	if c == nil {
		return
	}
	if c.config.RabbitMQ.URL == "" && os.Getenv("RABBITMQ_URL") == "" {
		logx.Info("notification consumer disabled: RabbitMQ URL is empty")
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
	cfg := c.config.RabbitMQ.Normalize()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stop:
			return
		default:
		}

		if err := c.consumeOnce(ctx, cfg); err != nil {
			logx.Errorf("notification consumer stopped: %v", err)
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

func (c *Consumer) consumeOnce(ctx context.Context, rabbitCfg pkgrabbitmq.Config) error {
	conn, err := pkgrabbitmq.Dial(rabbitCfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	consumeCh, err := conn.Channel()
	if err != nil {
		return err
	}
	defer consumeCh.Close()

	publishCh, err := conn.Channel()
	if err != nil {
		return err
	}
	defer publishCh.Close()

	if err := declareEventTopology(consumeCh, rabbitCfg.Exchange, c.config.Queue, c.config.BindingKey); err != nil {
		return err
	}
	if err := pkgrabbitmq.DeclareTopicExchange(publishCh, c.config.PushExchange); err != nil {
		return err
	}
	publisher, err := newConfirmedPublisher(publishCh, c.config.PushExchange)
	if err != nil {
		return err
	}
	if err := consumeCh.Qos(rabbitCfg.Prefetch, 0, false); err != nil {
		return err
	}
	deliveries, err := consumeCh.Consume(c.config.Queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	workers := c.config.WorkerCount
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-c.stop:
					return
				case delivery, ok := <-deliveries:
					if !ok {
						return
					}
					c.handleDelivery(ctx, publisher, delivery)
				}
			}
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.stop:
		return nil
	case <-done:
		return amqp.ErrClosed
	}
}

func declareEventTopology(ch *amqp.Channel, exchange, queue, bindingKey string) error {
	if exchange == "" {
		exchange = "beehive.im.events"
	}
	if err := pkgrabbitmq.DeclareTopicExchange(ch, exchange); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare notification queue: %w", err)
	}
	if err := ch.QueueBind(queue, bindingKey, exchange, false, nil); err != nil {
		return fmt.Errorf("bind notification queue: %w", err)
	}
	return nil
}

func (c *Consumer) handleDelivery(ctx context.Context, publisher *confirmedPublisher, delivery amqp.Delivery) {
	event, err := decodeMessageCreatedEvent(delivery.Body)
	if err != nil {
		logx.Errorf("notification event decode failed: %v", err)
		ack(delivery)
		return
	}

	claimed, err := c.claimEvent(ctx, event.EventID)
	if err != nil {
		logx.Errorf("notification dedupe claim failed: event_id=%s error=%v", event.EventID, err)
		nackRequeue(delivery)
		return
	}
	if !claimed {
		ack(delivery)
		return
	}

	success := false
	defer func() {
		if !success {
			c.releaseEvent(context.Background(), event.EventID)
		}
	}()

	if err := c.processEvent(ctx, publisher, event); err != nil {
		logx.Errorf("notification event process failed: event_id=%s conversation_id=%s error=%v", event.EventID, event.ConversationID, err)
		nackRequeue(delivery)
		return
	}
	success = true
	ack(delivery)
}

func (c *Consumer) processEvent(ctx context.Context, publisher *confirmedPublisher, event MessageCreatedEvent) error {
	recipients, err := c.deps.Conversation.ResolveMessageRecipients(ctx, &conversationservice.ResolveMessageRecipientsRequest{
		ConversationId: event.ConversationID,
		SenderId:       event.SenderID,
	})
	if err != nil {
		return fmt.Errorf("resolve message recipients rpc: %w", err)
	}
	if !recipients.GetAccepted() {
		logx.Infof("notification recipients skipped: event_id=%s conversation_id=%s code=%s", event.EventID, event.ConversationID, recipients.GetErrorCode())
		return nil
	}

	for _, userID := range recipients.GetUserIds() {
		routes, err := c.deps.Presence.GetLiveRoutes(ctx, &presenceservice.GetLiveRoutesRequest{UserId: userID})
		if err != nil {
			return fmt.Errorf("get live routes rpc: user_id=%s: %w", userID, err)
		}
		for _, route := range routes.GetRoutes() {
			if route == nil {
				continue
			}
			if route.GetUserId() == event.SenderID && route.GetDeviceId() == event.DeviceID {
				continue
			}
			if err := c.publishRoute(ctx, publisher, event, route); err != nil {
				_ = c.deps.Deliveries.RecordDelivery(ctx, deliveryFromRoute(event, route, repository.StatusFailed, err.Error()))
				return err
			}
			if err := c.deps.Deliveries.RecordDelivery(ctx, deliveryFromRoute(event, route, repository.StatusPushed, "")); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Consumer) publishRoute(ctx context.Context, publisher *confirmedPublisher, event MessageCreatedEvent, route *presenceservice.ConnectionMeta) error {
	body, err := buildEdgePush(event, route)
	if err != nil {
		return err
	}
	routingKey := "push.edge." + route.GetEdgeId()
	publishCtx, cancel := context.WithTimeout(ctx, c.config.PublishTimeout)
	defer cancel()
	if err := publisher.Publish(publishCtx, routingKey, body); err != nil {
		return fmt.Errorf("publish edge push: edge_id=%s conn_id=%s: %w", route.GetEdgeId(), route.GetConnId(), err)
	}
	return nil
}

func (c *Consumer) claimEvent(ctx context.Context, eventID string) (bool, error) {
	if c.deps.Redis == nil {
		return false, errors.New("redis client is not initialized")
	}
	key := "notify:dedupe:" + eventID
	return c.deps.Redis.SetNX(ctx, key, "1", c.config.DedupeTTL).Result()
}

func (c *Consumer) releaseEvent(ctx context.Context, eventID string) {
	if c.deps.Redis == nil || eventID == "" {
		return
	}
	_ = c.deps.Redis.Del(ctx, "notify:dedupe:"+eventID).Err()
}

type confirmedPublisher struct {
	ch        *amqp.Channel
	exchange  string
	confirms  <-chan amqp.Confirmation
	publishMu sync.Mutex
}

func newConfirmedPublisher(ch *amqp.Channel, exchange string) (*confirmedPublisher, error) {
	if err := ch.Confirm(false); err != nil {
		return nil, fmt.Errorf("enable rabbitmq publisher confirm: %w", err)
	}
	return &confirmedPublisher{
		ch:       ch,
		exchange: exchange,
		confirms: ch.NotifyPublish(make(chan amqp.Confirmation, 64)),
	}, nil
}

func (p *confirmedPublisher) Publish(ctx context.Context, routingKey string, body []byte) error {
	p.publishMu.Lock()
	defer p.publishMu.Unlock()

	if err := pkgrabbitmq.Publish(ctx, p.ch, p.exchange, routingKey, body); err != nil {
		return err
	}
	select {
	case confirm := <-p.confirms:
		if !confirm.Ack {
			return errors.New("rabbitmq publish was nack")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type MessageCreatedEvent struct {
	EventID        string          `json:"event_id"`
	EventType      string          `json:"event_type"`
	MessageID      string          `json:"message_id"`
	ConversationID string          `json:"conversation_id"`
	Seq            int64           `json:"seq"`
	SenderID       string          `json:"sender_id"`
	DeviceID       string          `json:"device_id"`
	ClientMsgID    string          `json:"client_msg_id"`
	ContentType    string          `json:"content_type"`
	Content        json.RawMessage `json:"content"`
	CreatedAt      string          `json:"created_at"`
}

func decodeMessageCreatedEvent(body []byte) (MessageCreatedEvent, error) {
	var event MessageCreatedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return MessageCreatedEvent{}, fmt.Errorf("unmarshal event: %w", err)
	}
	event.EventID = strings.TrimSpace(event.EventID)
	event.EventType = strings.TrimSpace(event.EventType)
	event.MessageID = strings.TrimSpace(event.MessageID)
	event.ConversationID = strings.TrimSpace(event.ConversationID)
	event.SenderID = strings.TrimSpace(event.SenderID)
	event.DeviceID = strings.TrimSpace(event.DeviceID)
	if event.EventID == "" || event.MessageID == "" || event.ConversationID == "" || event.Seq <= 0 || event.SenderID == "" {
		return MessageCreatedEvent{}, errors.New("message.created event missing required fields")
	}
	if event.EventType != "" && event.EventType != "message.created" {
		return MessageCreatedEvent{}, fmt.Errorf("unsupported event_type: %s", event.EventType)
	}
	if len(event.Content) == 0 {
		event.Content = json.RawMessage(`{}`)
	}
	return event, nil
}

type edgePushMessage struct {
	ConnID    string          `json:"conn_id"`
	SessionID string          `json:"session_id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

func buildEdgePush(event MessageCreatedEvent, route *presenceservice.ConnectionMeta) ([]byte, error) {
	payload, err := json.Marshal(map[string]any{
		"event_id":        event.EventID,
		"message_id":      event.MessageID,
		"conversation_id": event.ConversationID,
		"seq":             event.Seq,
		"sender_id":       event.SenderID,
		"device_id":       event.DeviceID,
		"client_msg_id":   event.ClientMsgID,
		"content_type":    event.ContentType,
		"content":         json.RawMessage(event.Content),
		"created_at":      event.CreatedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("encode edge push payload: %w", err)
	}
	body, err := json.Marshal(edgePushMessage{
		ConnID:    route.GetConnId(),
		SessionID: route.GetSessionId(),
		Type:      "message.new",
		Payload:   payload,
	})
	if err != nil {
		return nil, fmt.Errorf("encode edge push message: %w", err)
	}
	return body, nil
}

func deliveryFromRoute(event MessageCreatedEvent, route *presenceservice.ConnectionMeta, status, lastError string) repository.DeliveryInput {
	return repository.DeliveryInput{
		EventID:        event.EventID,
		ConversationID: event.ConversationID,
		UserID:         route.GetUserId(),
		DeviceID:       route.GetDeviceId(),
		EdgeID:         route.GetEdgeId(),
		ConnID:         route.GetConnId(),
		SessionID:      route.GetSessionId(),
		Status:         status,
		LastError:      lastError,
	}
}

func normalizeConfig(config Config) Config {
	if config.Queue == "" {
		config.Queue = defaultQueue
	}
	if config.BindingKey == "" {
		config.BindingKey = defaultBindingKey
	}
	if config.PushExchange == "" {
		config.PushExchange = defaultPushExchange
	}
	if config.WorkerCount <= 0 {
		config.WorkerCount = defaultWorkerCount
	}
	if config.DedupeTTL <= 0 {
		config.DedupeTTL = defaultDedupeTTL
	}
	if config.PublishTimeout <= 0 {
		config.PublishTimeout = defaultPublishTimout
	}
	return config
}

func ack(delivery amqp.Delivery) {
	if delivery.Acknowledger != nil {
		_ = delivery.Ack(false)
	}
}

func nackRequeue(delivery amqp.Delivery) {
	if delivery.Acknowledger != nil {
		_ = delivery.Nack(false, true)
	}
}
