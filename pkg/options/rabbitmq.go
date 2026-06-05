package options

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/pflag"
)

type RabbitMQOptions struct {
	Host                      string        `json:"host"                mapstructure:"host"`
	Port                      int           `json:"port"                mapstructure:"port"`
	User                      string        `json:"user"                mapstructure:"user"`
	Password                  string        `json:"password"            mapstructure:"password"`
	VirtualHost               string        `json:"virtual-host"        mapstructure:"virtual-host"`
	Exchange                  string        `json:"exchange"            mapstructure:"exchange"`
	Queue                     string        `json:"queue"               mapstructure:"queue"`
	InstanceID                string        `json:"instance-id"         mapstructure:"instance-id"`
	Prefetch                  int           `json:"prefetch"            mapstructure:"prefetch"`
	PublishTimeout            time.Duration `json:"publish-timeout"     mapstructure:"publish-timeout"`
	ConsumeConcurrency        int           `json:"consume-concurrency" mapstructure:"consume-concurrency"`
	PublishWorkerConcurrency  int           `json:"publish-worker-concurrency" mapstructure:"publish-worker-concurrency"`
	PublishBatchSize          int           `json:"publish-batch-size"  mapstructure:"publish-batch-size"`
	DeliveryMaxAttempts       int           `json:"delivery-max-attempts" mapstructure:"delivery-max-attempts"`
	PresenceTTL               time.Duration `json:"presence-ttl"        mapstructure:"presence-ttl"`
	PresenceHeartbeatInterval time.Duration `json:"presence-heartbeat-interval" mapstructure:"presence-heartbeat-interval"`
	ReconnectMinBackoff       time.Duration `json:"reconnect-min-backoff" mapstructure:"reconnect-min-backoff"`
	ReconnectMaxBackoff       time.Duration `json:"reconnect-max-backoff" mapstructure:"reconnect-max-backoff"`
}

func NewRabbitMQOptions() *RabbitMQOptions {
	return &RabbitMQOptions{}
}

func (r *RabbitMQOptions) Validate() error {
	var err error
	if r.Host == "" {
		err = errors.Join(err, fmt.Errorf("host is required"))
	}
	if r.Port == 0 {
		err = errors.Join(err, fmt.Errorf("port is required"))
	} else if r.Port < minPort || r.Port > maxPort {
		err = errors.Join(err, fmt.Errorf("port must be between %d and %d inclusive, got %d", minPort, maxPort, r.Port))
	}
	if r.User == "" {
		err = errors.Join(err, fmt.Errorf("user is required"))
	}
	if r.Password == "" {
		err = errors.Join(err, fmt.Errorf("password is required"))
	}
	if r.VirtualHost == "" {
		err = errors.Join(err, fmt.Errorf("virtual-host is required"))
	}
	if r.Exchange == "" {
		err = errors.Join(err, fmt.Errorf("exchange is required"))
	}
	if r.Queue == "" {
		err = errors.Join(err, fmt.Errorf("queue is required"))
	}
	if r.InstanceID == "" {
		err = errors.Join(err, fmt.Errorf("instance-id is required"))
	}
	if r.Prefetch <= 0 {
		err = errors.Join(err, fmt.Errorf("prefetch must be > 0, got %d", r.Prefetch))
	}
	if r.PublishTimeout <= 0 {
		err = errors.Join(err, fmt.Errorf("publish-timeout must be > 0, got %s", r.PublishTimeout))
	}
	if r.ConsumeConcurrency <= 0 {
		err = errors.Join(err, fmt.Errorf("consume-concurrency must be > 0, got %d", r.ConsumeConcurrency))
	}
	if r.PublishWorkerConcurrency <= 0 {
		err = errors.Join(err, fmt.Errorf("publish-worker-concurrency must be > 0, got %d", r.PublishWorkerConcurrency))
	}
	if r.PublishBatchSize <= 0 {
		err = errors.Join(err, fmt.Errorf("publish-batch-size must be > 0, got %d", r.PublishBatchSize))
	}
	if r.DeliveryMaxAttempts <= 0 {
		err = errors.Join(err, fmt.Errorf("delivery-max-attempts must be > 0, got %d", r.DeliveryMaxAttempts))
	}
	if r.PresenceTTL <= 0 {
		err = errors.Join(err, fmt.Errorf("presence-ttl must be > 0, got %s", r.PresenceTTL))
	}
	if r.PresenceHeartbeatInterval <= 0 {
		err = errors.Join(err, fmt.Errorf("presence-heartbeat-interval must be > 0, got %s", r.PresenceHeartbeatInterval))
	} else if r.PresenceHeartbeatInterval >= r.PresenceTTL {
		err = errors.Join(err, fmt.Errorf("presence-heartbeat-interval must be < presence-ttl"))
	}
	if r.ReconnectMinBackoff <= 0 {
		err = errors.Join(err, fmt.Errorf("reconnect-min-backoff must be > 0, got %s", r.ReconnectMinBackoff))
	}
	if r.ReconnectMaxBackoff < r.ReconnectMinBackoff {
		err = errors.Join(err, fmt.Errorf("reconnect-max-backoff must be >= reconnect-min-backoff"))
	}
	return err
}

func (r *RabbitMQOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&r.Host, "rabbitmq-host", "127.0.0.1", "RabbitMQ hostname or IP address")
	fs.IntVar(&r.Port, "rabbitmq-port", 5672, "RabbitMQ server TCP port")
	fs.StringVar(&r.User, "rabbitmq-user", "guest", "RabbitMQ user name")
	fs.StringVar(&r.Password, "rabbitmq-password", "guest", "RabbitMQ password")
	fs.StringVar(&r.VirtualHost, "rabbitmq-virtual-host", "/", "RabbitMQ virtual host")
	fs.StringVar(&r.Exchange, "rabbitmq-exchange", "im.events", "RabbitMQ topic exchange for IM events")
	fs.StringVar(&r.Queue, "rabbitmq-queue", "im.message.dispatch", "RabbitMQ queue for local message dispatch")
	fs.StringVar(&r.InstanceID, "rabbitmq-instance-id", "dev-instance", "Unique application instance id for per-instance dispatch queues")
	fs.IntVar(&r.Prefetch, "rabbitmq-prefetch", 64, "RabbitMQ consumer prefetch count")
	fs.DurationVar(&r.PublishTimeout, "rabbitmq-publish-timeout", 5*time.Second, "RabbitMQ publish confirmation timeout")
	fs.IntVar(&r.ConsumeConcurrency, "rabbitmq-consume-concurrency", 4, "RabbitMQ message consumer worker count")
	fs.IntVar(&r.PublishWorkerConcurrency, "rabbitmq-publish-worker-concurrency", 2, "Message delivery publisher worker count")
	fs.IntVar(&r.PublishBatchSize, "rabbitmq-publish-batch-size", 100, "Message delivery publisher batch size")
	fs.IntVar(&r.DeliveryMaxAttempts, "rabbitmq-delivery-max-attempts", 5, "Maximum publish attempts before marking a delivery failed")
	fs.DurationVar(&r.PresenceTTL, "rabbitmq-presence-ttl", 90*time.Second, "Redis presence TTL for websocket sessions")
	fs.DurationVar(&r.PresenceHeartbeatInterval, "rabbitmq-presence-heartbeat-interval", 30*time.Second, "Redis presence heartbeat refresh interval")
	fs.DurationVar(&r.ReconnectMinBackoff, "rabbitmq-reconnect-min-backoff", time.Second, "RabbitMQ reconnect minimum backoff")
	fs.DurationVar(&r.ReconnectMaxBackoff, "rabbitmq-reconnect-max-backoff", 30*time.Second, "RabbitMQ reconnect maximum backoff")
}
