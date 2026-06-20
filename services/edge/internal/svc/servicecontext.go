// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"context"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/presence"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/push"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/ticket"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/upstream"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/wsproxy"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/gatewayservice"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/presenceservice"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config   config.Config
	Gateway  gatewayservice.GatewayService
	Presence presence.Client
	Tickets  *ticket.Store
	Proxy    *wsproxy.Proxy
	Router   *upstream.Router
	Push     *push.Consumer
}

func NewServiceContext(c config.Config) *ServiceContext {
	ttl := time.Duration(c.Ticket.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 30 * time.Second
	}

	tickets := ticket.NewStore(ttl)
	gateway := gatewayservice.NewGatewayService(zrpc.MustNewClient(c.Gateway))
	etcdConfig := c.Registry
	if etcdConfig.Env == "" {
		etcdConfig.Env = c.Env
	}
	if etcdConfig.Prefix == "" {
		etcdConfig.Prefix = c.RegistryPrefix
	}
	router := upstream.NewRouter(upstream.Config{
		Etcd:       etcdConfig,
		ClientConf: c.Gateway,
		Static:     gateway,
		StaticID:   "static-gateway",
	})
	router.Start(context.Background())

	presenceClient := presence.NewRPCClient(
		presenceservice.NewPresenceService(zrpc.MustNewClient(c.Presence)),
		c.PresenceTTLSeconds,
	)
	proxy := wsproxy.NewProxy(wsproxy.Config{
		EdgeID:          c.EdgeID,
		WriteBufferSize: c.WebSocket.WriteBufferSize,
		ReadLimitBytes:  c.WebSocket.ReadLimitBytes,
		Tickets:         tickets,
		GatewayRouter:   router,
		Presence:        presenceClient,
	})
	pushConsumer := push.NewConsumer(c.EdgeID, c.RabbitMQ, proxy)
	pushConsumer.Start(context.Background())

	return &ServiceContext{
		Config:   c,
		Gateway:  gateway,
		Presence: presenceClient,
		Tickets:  tickets,
		Proxy:    proxy,
		Router:   router,
		Push:     pushConsumer,
	}
}

func (s *ServiceContext) Close() {
	if s.Push != nil {
		s.Push.Stop()
	}
	if s.Router != nil {
		s.Router.Close()
	}
}
