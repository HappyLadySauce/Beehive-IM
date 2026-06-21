package config

import (
	"github.com/HappyLadySauce/Beehive-IM/pkg/postgres"
	pkgrabbitmq "github.com/HappyLadySauce/Beehive-IM/pkg/rabbitmq"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Env          string `json:",default=dev"`
	Postgres     postgres.Config
	RabbitMQ     pkgrabbitmq.Config
	Conversation zrpc.RpcClientConf
	Outbox       OutboxConf
}

type OutboxConf struct {
	Enabled          bool  `json:",default=true"`
	BatchSize        int   `json:",default=32"`
	PollIntervalMs   int64 `json:",default=500"`
	LockTTLSeconds   int64 `json:",default=30"`
	MaxAttempts      int   `json:",default=20"`
	RetryBaseDelayMs int64 `json:",default=500"`
	RetryMaxDelayMs  int64 `json:",default=30000"`
	PublishTimeoutMs int64 `json:",default=5000"`
}
