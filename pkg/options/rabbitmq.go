package options

import (
	"errors"
	"fmt"

	"github.com/spf13/pflag"
)

type RabbitMQOptions struct {
	Host            string        `json:"host"              mapstructure:"host"`
	Port            int           `json:"port"              mapstructure:"port"`
	User            string        `json:"user"              mapstructure:"user"`
	Password        string        `json:"password"          mapstructure:"password"`
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
	return err
}

func (r *RabbitMQOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&r.Host, "rabbitmq-host", "127.0.0.1", "RabbitMQ hostname or IP address")
	fs.IntVar(&r.Port, "rabbitmq-port", 5672, "RabbitMQ server TCP port")
	fs.StringVar(&r.User, "rabbitmq-user", "guest", "RabbitMQ user name")
	fs.StringVar(&r.Password, "rabbitmq-password", "guest", "RabbitMQ password")
}