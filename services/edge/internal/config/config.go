// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	EdgeID    string `json:",default=edge-dev"`
	Ticket    TicketConf
	WebSocket WebSocketConf
	Gateway   zrpc.RpcClientConf
}

type TicketConf struct {
	TTLSeconds int64 `json:",default=30"`
}

type WebSocketConf struct {
	WriteBufferSize int   `json:",default=64"`
	ReadLimitBytes  int64 `json:",default=65536"`
}
