package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf
	GatewayID   string `json:",default=gateway-dev"`
	MaxSessions int    `json:",default=20000"`
}
