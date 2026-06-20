package svc

import (
	"context"
	"sync"
	"time"

	pkgetcd "github.com/HappyLadySauce/Beehive-IM/pkg/etcd"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/session"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config       config.Config
	Sessions     *session.Manager
	registration *pkgetcd.Registration
	etcdClient   interface{ Close() error }
	stopRegistry chan struct{}
	mu           sync.RWMutex
	status       string
}

func NewServiceContext(c config.Config) *ServiceContext {
	ctx := &ServiceContext{
		Config:       c,
		Sessions:     session.NewManager(c.GatewayID, c.MaxSessions),
		stopRegistry: make(chan struct{}),
		status:       "online",
	}
	ctx.startRegistry()
	return ctx
}

func (s *ServiceContext) Close() {
	select {
	case <-s.stopRegistry:
	default:
		close(s.stopRegistry)
	}
	if s.registration != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.registration.Close(closeCtx); err != nil {
			logx.Errorf("gateway registry close failed: %v", err)
		}
	}
	if s.etcdClient != nil {
		if err := s.etcdClient.Close(); err != nil {
			logx.Errorf("gateway etcd client close failed: %v", err)
		}
	}
}

func (s *ServiceContext) Status() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.status == "" {
		return "online"
	}
	return s.status
}

func (s *ServiceContext) IsDraining() bool {
	return s.Status() == "draining"
}

func (s *ServiceContext) EnterDraining(ctx context.Context) error {
	s.mu.Lock()
	s.status = "draining"
	s.mu.Unlock()

	if s.registration == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.registration.Update(ctx, s.registryNode()); err != nil {
		logx.Errorf("gateway registry draining update failed: %v", err)
		return err
	}
	return nil
}

func (s *ServiceContext) startRegistry() {
	cfg := pkgetcd.Config{
		Endpoints: s.Config.Etcd.Hosts,
		Prefix:    s.Config.RegistryPrefix,
		Env:       s.Config.Env,
	}
	cli, err := pkgetcd.NewClient(cfg)
	if err != nil {
		logx.Errorf("gateway registry disabled: %v", err)
		return
	}
	s.etcdClient = cli

	node := s.registryNode()
	regCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	reg, err := pkgetcd.RegisterService(regCtx, cli, cfg, "gateway", node)
	cancel()
	if err != nil {
		logx.Errorf("gateway registry disabled: %v", err)
		return
	}
	s.registration = reg

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				updateCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				if err := reg.Update(updateCtx, s.registryNode()); err != nil {
					logx.Errorf("gateway registry update failed: %v", err)
				}
				cancel()
			case <-s.stopRegistry:
				return
			}
		}
	}()
}

func (s *ServiceContext) registryNode() pkgetcd.ServiceNode {
	return pkgetcd.ServiceNode{
		SchemaVersion: 1,
		InstanceID:    s.Sessions.GatewayID(),
		Service:       "gateway",
		Address:       s.Config.UpstreamAddr,
		Status:        s.Status(),
		SessionCount:  s.Sessions.Count(),
		MaxSessions:   s.Config.MaxSessions,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
}
