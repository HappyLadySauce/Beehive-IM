// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	pkgetcd "github.com/HappyLadySauce/Beehive-IM/pkg/etcd"
	pkgrabbitmq "github.com/HappyLadySauce/Beehive-IM/pkg/rabbitmq"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Env                string `json:",default=dev"`
	RegistryPrefix     string `json:",default=/beehive-im"`
	EdgeID             string `json:",default=edge-dev"`
	PresenceTTLSeconds int64  `json:",default=90"`
	Ticket             TicketConf
	WebSocket          WebSocketConf
	Gateway            zrpc.RpcClientConf
	GatewayRecovery    GatewayRecoveryConf
	Presence           zrpc.RpcClientConf
	Message            zrpc.RpcClientConf
	Registry           pkgetcd.Config
	RabbitMQ           pkgrabbitmq.Config
}

type TicketConf struct {
	TTLSeconds int64 `json:",default=30"`
}

type WebSocketConf struct {
	WriteBufferSize int   `json:",default=64"`
	ReadLimitBytes  int64 `json:",default=65536"`
}

type GatewayRecoveryConf struct {
	MaxAttempts int     `json:",default=3"`
	WindowMs    int64   `json:",default=5000"`
	BackoffMs   []int64 `json:",optional"`
	IsolationMs int64   `json:",default=10000"`
}
