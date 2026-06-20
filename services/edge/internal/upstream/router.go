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

// Router picks an internal Gateway for Edge connections.
// Router 为 Edge 连接选择内网 Gateway。
type Router struct {
	mu          sync.RWMutex
	etcdClient  *clientv3.Client
	etcdConfig  pkgetcd.Config
	clientConf  zrpc.RpcClientConf
	static      gatewayservice.GatewayService
	staticID    string
	nodes       []pkgetcd.ServiceNode
	stop        chan struct{}
	startedOnce sync.Once
}

type Config struct {
	Etcd       pkgetcd.Config
	ClientConf zrpc.RpcClientConf
	Static     gatewayservice.GatewayService
	StaticID   string
}

func NewRouter(c Config) *Router {
	router := &Router{
		etcdConfig: c.Etcd,
		clientConf: c.ClientConf,
		static:     c.Static,
		staticID:   c.StaticID,
		stop:       make(chan struct{}),
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
		r.reload(ctx)
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
	if r.static != nil && !containsID(excludedGatewayIDs, r.staticID) {
		return r.static, r.staticID, nil
	}
	return nil, "", ErrNoGatewayAvailable
}

func (r *Router) pickNode(excludedGatewayIDs ...string) (pkgetcd.ServiceNode, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	candidates := make([]pkgetcd.ServiceNode, 0, len(r.nodes))
	for _, node := range r.nodes {
		if containsID(excludedGatewayIDs, node.InstanceID) {
			continue
		}
		if node.Status != "online" || node.Address == "" {
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

func (r *Router) reload(ctx context.Context) {
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
	r.mu.Lock()
	r.nodes = nodes
	r.mu.Unlock()
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
			r.reload(ctx)
		case <-r.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}
