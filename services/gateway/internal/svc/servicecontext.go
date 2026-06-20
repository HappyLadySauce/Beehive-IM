package svc

import (
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/session"
)

type ServiceContext struct {
	Config   config.Config
	Sessions *session.Manager
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:   c,
		Sessions: session.NewManager(c.GatewayID, c.MaxSessions),
	}
}
