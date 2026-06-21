package config

import (
	"github.com/HappyLadySauce/Beehive-IM/pkg/postgres"
	"github.com/HappyLadySauce/Beehive-IM/pkg/rabbitmq"
	"github.com/HappyLadySauce/Beehive-IM/pkg/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Env          string `json:",default=dev"`
	Postgres     postgres.Config
	Redis        redis.Config
	RabbitMQ     rabbitmq.Config
	Conversation zrpc.RpcClientConf
	Presence     zrpc.RpcClientConf
	Worker       WorkerConf
}

type WorkerConf struct {
	Enabled               bool   `json:",default=true"`
	Queue                 string `json:",default=notification.message.events"`
	BindingKey            string `json:",default=message.created.#"`
	PushExchange          string `json:",default=beehive.im.push"`
	WorkerCount           int    `json:",default=2"`
	DedupeTTLSeconds      int64  `json:",default=86400"`
	PublishTimeoutSeconds int64  `json:",default=5"`
}
