package upstream

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	pkgetcd "github.com/HappyLadySauce/Beehive-IM/pkg/etcd"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/gatewayservice"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
)

var ErrNoGatewayAvailable = errors.New("no gateway available")

const (
	StatusOnline   = "online"
	StatusDraining = "draining"
	StatusDeleted  = "deleted"
)

// GatewayEvent describes one Gateway registry state transition.
// GatewayEvent 描述一个 Gateway 注册状态变更事件。
type GatewayEvent struct {
	GatewayID string
	Status    string
}

// GatewayEventHandler handles Gateway registry state transitions.
// GatewayEventHandler 处理 Gateway 注册状态变更。
type GatewayEventHandler func(GatewayEvent)

// Router picks an internal Gateway for Edge connections.
// Router 为 Edge 连接选择内网 Gateway。
type Router struct {
	mu                sync.RWMutex
	etcdClient        *clientv3.Client
	etcdConfig        pkgetcd.Config
	clientConf        zrpc.RpcClientConf
	static            gatewayservice.GatewayService
	staticID          string
	nodes             []pkgetcd.ServiceNode
	isolatedUntil     map[string]time.Time
	isolationDuration time.Duration
	handlers          []GatewayEventHandler
	stop              chan struct{}
	startedOnce       sync.Once
}

type Config struct {
	Etcd              pkgetcd.Config
	ClientConf        zrpc.RpcClientConf
	Static            gatewayservice.GatewayService
	StaticID          string
	IsolationDuration time.Duration
}

func NewRouter(c Config) *Router {
	if c.IsolationDuration <= 0 {
		c.IsolationDuration = 10 * time.Second
	}
	router := &Router{
		etcdConfig:        c.Etcd,
		clientConf:        c.ClientConf,
		static:            c.Static,
		staticID:          c.StaticID,
		isolatedUntil:     make(map[string]time.Time),
		isolationDuration: c.IsolationDuration,
		stop:              make(chan struct{}),
	}
	if cli, err := pkgetcd.NewClient(c.Etcd); err != nil {
		logx.Errorf("edge gateway registry disabled: %v", err)
	} else {
		router.etcdClient = cli
	}
	return router
}

func (r *Router) Start(ctx context.Context) {
	r.startedOnce.Do(func() {
		if r.etcdClient == nil {
			return
		}
		r.reload(ctx, false)
		go r.watch(ctx)
	})
}

func (r *Router) Close() {
	select {
	case <-r.stop:
	default:
		close(r.stop)
	}
	if r.etcdClient != nil {
		_ = r.etcdClient.Close()
	}
}

func (r *Router) Subscribe(handler GatewayEventHandler) {
	if handler == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers = append(r.handlers, handler)
}

func (r *Router) MarkFailed(gatewayID string) {
	if gatewayID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.isolatedUntil[gatewayID] = time.Now().Add(r.isolationDuration)
}

func (r *Router) Pick(ctx context.Context, excludedGatewayIDs ...string) (gatewayservice.GatewayService, string, error) {
	node, ok := r.pickNode(excludedGatewayIDs...)
	if ok {
		conf := r.clientConf
		conf.Endpoints = []string{node.Address}
		conf.Etcd.Hosts = nil
		conf.Etcd.Key = ""
		if conf.Timeout == 0 {
			conf.Timeout = 2000
		}
		if !conf.NonBlock {
			conf.NonBlock = true
		}
		return gatewayservice.NewGatewayService(zrpc.MustNewClient(conf)), node.InstanceID, nil
	}
	if r.static != nil && !containsID(excludedGatewayIDs, r.staticID) && !r.isIsolated(r.staticID) {
		return r.static, r.staticID, nil
	}
	return nil, "", ErrNoGatewayAvailable
}

func (r *Router) pickNode(excludedGatewayIDs ...string) (pkgetcd.ServiceNode, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	candidates := make([]pkgetcd.ServiceNode, 0, len(r.nodes))
	now := time.Now()
	for _, node := range r.nodes {
		if containsID(excludedGatewayIDs, node.InstanceID) {
			continue
		}
		if r.isIsolatedLocked(node.InstanceID, now) {
			continue
		}
		if node.Status != StatusOnline || node.Address == "" {
			continue
		}
		if node.MaxSessions > 0 && node.SessionCount >= node.MaxSessions {
			continue
		}
		candidates = append(candidates, node)
	}
	if len(candidates) == 0 {
		return pkgetcd.ServiceNode{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].SessionCount < candidates[j].SessionCount
	})
	return candidates[0], true
}

func (r *Router) isIsolated(gatewayID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.isIsolatedLocked(gatewayID, time.Now())
}

func (r *Router) isIsolatedLocked(gatewayID string, now time.Time) bool {
	if gatewayID == "" || r.isolatedUntil == nil {
		return false
	}
	until, ok := r.isolatedUntil[gatewayID]
	return ok && until.After(now)
}

func containsID(ids []string, target string) bool {
	if target == "" {
		return false
	}
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func (r *Router) reload(ctx context.Context, emitEvents bool) {
	if r.etcdClient == nil {
		return
	}
	reloadCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	nodes, err := pkgetcd.ListService(reloadCtx, r.etcdClient, r.etcdConfig, "gateway")
	if err != nil {
		logx.Errorf("edge gateway registry reload failed: %v", err)
		return
	}

	events := r.gatewayEvents(nodes, emitEvents)
	r.mu.Lock()
	r.nodes = nodes
	r.mu.Unlock()
	r.emit(events)
}

func (r *Router) watch(ctx context.Context) {
	prefix := pkgetcd.ServicePrefix(r.etcdConfig.Normalize().Prefix, r.etcdConfig.Normalize().Env, "gateway")
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	watchCh := r.etcdClient.Watch(watchCtx, prefix, clientv3.WithPrefix())
	for {
		select {
		case resp, ok := <-watchCh:
			if !ok {
				return
			}
			if resp.Err() != nil {
				logx.Errorf("edge gateway registry watch failed: %v", resp.Err())
				continue
			}
			r.reload(ctx, true)
		case <-r.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (r *Router) gatewayEvents(next []pkgetcd.ServiceNode, emitEvents bool) []GatewayEvent {
	if !emitEvents {
		return nil
	}

	r.mu.RLock()
	current := make(map[string]pkgetcd.ServiceNode, len(r.nodes))
	for _, node := range r.nodes {
		current[node.InstanceID] = node
	}
	r.mu.RUnlock()

	nextByID := make(map[string]pkgetcd.ServiceNode, len(next))
	for _, node := range next {
		nextByID[node.InstanceID] = node
	}

	events := make([]GatewayEvent, 0)
	for gatewayID, oldNode := range current {
		nextNode, ok := nextByID[gatewayID]
		if !ok {
			events = append(events, GatewayEvent{GatewayID: gatewayID, Status: StatusDeleted})
			continue
		}
		if oldNode.Status != StatusDraining && nextNode.Status == StatusDraining {
			events = append(events, GatewayEvent{GatewayID: gatewayID, Status: StatusDraining})
		}
	}
	return events
}

func (r *Router) emit(events []GatewayEvent) {
	if len(events) == 0 {
		return
	}

	r.mu.RLock()
	handlers := append([]GatewayEventHandler(nil), r.handlers...)
	r.mu.RUnlock()

	for _, event := range events {
		for _, handler := range handlers {
			handler(event)
		}
	}
}
