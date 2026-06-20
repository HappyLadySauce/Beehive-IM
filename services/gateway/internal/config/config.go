package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf
	GatewayID      string `json:",default=gateway-dev"`
	Env            string `json:",default=dev"`
	RegistryPrefix string `json:",default=/beehive-im"`
	UpstreamAddr   string `json:",default=127.0.0.1:9100"`
	MaxSessions    int    `json:",default=20000"`
}
