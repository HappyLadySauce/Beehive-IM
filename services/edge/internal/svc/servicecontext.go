// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/presence"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/ticket"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/wsproxy"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/gatewayservice"
	"github.com/zeromicro/go-zero/zrpc"
	"time"
)

type ServiceContext struct {
	Config   config.Config
	Gateway  gatewayservice.GatewayService
	Presence presence.Client
	Tickets  *ticket.Store
	Proxy    *wsproxy.Proxy
}

func NewServiceContext(c config.Config) *ServiceContext {
	ttl := time.Duration(c.Ticket.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 30 * time.Second
	}

	tickets := ticket.NewStore(ttl)
	gateway := gatewayservice.NewGatewayService(zrpc.MustNewClient(c.Gateway))
	presenceClient := presence.NewNoopClient()

	return &ServiceContext{
		Config:   c,
		Gateway:  gateway,
		Presence: presenceClient,
		Tickets:  tickets,
		Proxy: wsproxy.NewProxy(wsproxy.Config{
			EdgeID:          c.EdgeID,
			WriteBufferSize: c.WebSocket.WriteBufferSize,
			ReadLimitBytes:  c.WebSocket.ReadLimitBytes,
			Tickets:         tickets,
			Gateway:         gateway,
			Presence:        presenceClient,
		}),
	}
}
