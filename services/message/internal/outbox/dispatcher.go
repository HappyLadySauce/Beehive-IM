package outbox

import (
	"context"
	"math"
	"sync"
	"time"

	pkgrabbitmq "github.com/HappyLadySauce/Beehive-IM/pkg/rabbitmq"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/repository"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"
)

const defaultExchange = "beehive.im.events"

// Store is the persistence boundary required by the dispatcher.
// Store 是 dispatcher 依赖的持久化边界。
type Store interface {
	FetchPendingOutbox(context.Context, int, time.Duration) ([]repository.OutboxEvent, error)
	MarkOutboxPublished(context.Context, string) error
	MarkOutboxFailed(context.Context, string, error, int, time.Duration) error
}

// Config controls outbox polling, locking, retry, and RabbitMQ publishing.
// Config 控制 outbox 轮询、锁定、重试和 RabbitMQ 发布。
type Config struct {
	BatchSize      int
	PollInterval   time.Duration
	LockTTL        time.Duration
	MaxAttempts    int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	PublishTimeout time.Duration
	RabbitMQ       pkgrabbitmq.Config
}

// Dispatcher publishes committed outbox events to RabbitMQ asynchronously.
// Dispatcher 异步将已提交 outbox 事件发布到 RabbitMQ。
type Dispatcher struct {
	config           Config
	store            Store
	publisherFactory func(pkgrabbitmq.Config) (Publisher, error)
	stop             chan struct{}
	once             sync.Once
}

// Publisher publishes one outbox event payload.
// Publisher 发布单条 outbox 事件载荷。
type Publisher interface {
	Publish(context.Context, string, []byte) error
	Close()
}

func NewDispatcher(config Config, store Store) *Dispatcher {
	config = normalizeConfig(config)
	return &Dispatcher{
		config:           config,
		store:            store,
		publisherFactory: newRabbitPublisher,
		stop:             make(chan struct{}),
	}
}

func (d *Dispatcher) Start(ctx context.Context) {
	if d == nil || d.store == nil {
		return
	}
	d.once.Do(func() {
		go d.run(ctx)
	})
}

func (d *Dispatcher) Stop() {
	if d == nil {
		return
	}
	select {
	case <-d.stop:
	default:
		close(d.stop)
	}
}

func (d *Dispatcher) run(ctx context.Context) {
	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()

	for {
		if d.dispatchOnce(ctx) {
			continue
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		case <-d.stop:
			return
		}
	}
}

func (d *Dispatcher) dispatchOnce(ctx context.Context) bool {
	events, err := d.store.FetchPendingOutbox(ctx, d.config.BatchSize, d.config.LockTTL)
	if err != nil {
		logx.Errorf("message outbox fetch failed: %v", err)
		return false
	}
	if len(events) == 0 {
		return false
	}

	publisher, err := d.publisherFactory(d.config.RabbitMQ)
	if err != nil {
		logx.Errorf("message outbox publisher unavailable: %v", err)
		for _, event := range events {
			d.markFailed(ctx, event, err)
		}
		return false
	}
	defer publisher.Close()

	for _, event := range events {
		publishCtx, cancel := context.WithTimeout(ctx, d.config.PublishTimeout)
		err = publisher.Publish(publishCtx, event.RoutingKey, event.PayloadJSON)
		cancel()
		if err != nil {
			logx.Errorf("message outbox publish failed: event_id=%s routing_key=%s error=%v", event.EventID, event.RoutingKey, err)
			d.markFailed(ctx, event, err)
			continue
		}
		if markErr := d.store.MarkOutboxPublished(ctx, event.EventID); markErr != nil {
			logx.Errorf("message outbox mark published failed: event_id=%s error=%v", event.EventID, markErr)
		}
	}
	return len(events) >= d.config.BatchSize
}

func (d *Dispatcher) markFailed(ctx context.Context, event repository.OutboxEvent, err error) {
	delay := d.retryDelay(event.Attempts)
	if markErr := d.store.MarkOutboxFailed(ctx, event.EventID, err, d.config.MaxAttempts, delay); markErr != nil {
		logx.Errorf("message outbox mark failed failed: event_id=%s error=%v", event.EventID, markErr)
	}
}

func (d *Dispatcher) retryDelay(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	pow := math.Pow(2, float64(attempts))
	delay := time.Duration(float64(d.config.RetryBaseDelay) * pow)
	if delay > d.config.RetryMaxDelay {
		return d.config.RetryMaxDelay
	}
	if delay <= 0 {
		return time.Second
	}
	return delay
}

type rabbitPublisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	confirms <-chan amqp.Confirmation
	exchange string
}

func newRabbitPublisher(config pkgrabbitmq.Config) (Publisher, error) {
	cfg := config.Normalize()
	if cfg.Exchange == "" || cfg.Exchange == "beehive.im.push" {
		cfg.Exchange = defaultExchange
	}
	conn, err := pkgrabbitmq.Dial(cfg)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := pkgrabbitmq.DeclareTopicExchange(ch, cfg.Exchange); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	return &rabbitPublisher{conn: conn, channel: ch, confirms: confirms, exchange: cfg.Exchange}, nil
}

func (p *rabbitPublisher) Publish(ctx context.Context, routingKey string, payload []byte) error {
	if err := pkgrabbitmq.Publish(ctx, p.channel, p.exchange, routingKey, payload); err != nil {
		return err
	}
	select {
	case confirm := <-p.confirms:
		if !confirm.Ack {
			return amqp.ErrClosed
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *rabbitPublisher) Close() {
	if p == nil {
		return
	}
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
}

func normalizeConfig(c Config) Config {
	if c.BatchSize <= 0 {
		c.BatchSize = 32
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 500 * time.Millisecond
	}
	if c.LockTTL <= 0 {
		c.LockTTL = 30 * time.Second
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 20
	}
	if c.RetryBaseDelay <= 0 {
		c.RetryBaseDelay = 500 * time.Millisecond
	}
	if c.RetryMaxDelay <= 0 {
		c.RetryMaxDelay = 30 * time.Second
	}
	if c.PublishTimeout <= 0 {
		c.PublishTimeout = 5 * time.Second
	}
	return c
}
