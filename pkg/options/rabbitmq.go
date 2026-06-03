package options

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/pflag"
)

type RabbitMQOptions struct {
	Host                string        `json:"host"                mapstructure:"host"`
	Port                int           `json:"port"                mapstructure:"port"`
	User                string        `json:"user"                mapstructure:"user"`
	Password            string        `json:"password"            mapstructure:"password"`
	VirtualHost         string        `json:"virtual-host"        mapstructure:"virtual-host"`
	Exchange            string        `json:"exchange"            mapstructure:"exchange"`
	Queue               string        `json:"queue"               mapstructure:"queue"`
	Prefetch            int           `json:"prefetch"            mapstructure:"prefetch"`
	PublishTimeout      time.Duration `json:"publish-timeout"     mapstructure:"publish-timeout"`
	ConsumeConcurrency  int           `json:"consume-concurrency" mapstructure:"consume-concurrency"`
	ReconnectMinBackoff time.Duration `json:"reconnect-min-backoff" mapstructure:"reconnect-min-backoff"`
	ReconnectMaxBackoff time.Duration `json:"reconnect-max-backoff" mapstructure:"reconnect-max-backoff"`
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
	if r.Prefetch <= 0 {
		err = errors.Join(err, fmt.Errorf("prefetch must be > 0, got %d", r.Prefetch))
	}
	if r.PublishTimeout <= 0 {
		err = errors.Join(err, fmt.Errorf("publish-timeout must be > 0, got %s", r.PublishTimeout))
	}
	if r.ConsumeConcurrency <= 0 {
		err = errors.Join(err, fmt.Errorf("consume-concurrency must be > 0, got %d", r.ConsumeConcurrency))
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
	fs.IntVar(&r.Prefetch, "rabbitmq-prefetch", 64, "RabbitMQ consumer prefetch count")
	fs.DurationVar(&r.PublishTimeout, "rabbitmq-publish-timeout", 5*time.Second, "RabbitMQ publish confirmation timeout")
	fs.IntVar(&r.ConsumeConcurrency, "rabbitmq-consume-concurrency", 4, "RabbitMQ message consumer worker count")
	fs.DurationVar(&r.ReconnectMinBackoff, "rabbitmq-reconnect-min-backoff", time.Second, "RabbitMQ reconnect minimum backoff")
	fs.DurationVar(&r.ReconnectMaxBackoff, "rabbitmq-reconnect-max-backoff", 30*time.Second, "RabbitMQ reconnect maximum backoff")
}
