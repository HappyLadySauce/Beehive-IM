package etcd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	defaultEndpoint           = "127.0.0.1:2379"
	defaultDialTimeoutSeconds = 3
	defaultLeaseTTLSeconds    = 30
)

// Config describes etcd client and service registry settings.
// Config 描述 etcd 客户端与服务注册配置。
type Config struct {
	Endpoints              []string `json:",optional"`
	Prefix                 string   `json:",default=/beehive-im"`
	Env                    string   `json:",default=dev"`
	DialTimeoutSeconds     int64    `json:",default=3"`
	LeaseTTLSeconds        int64    `json:",default=30"`
	PreferEnvWhenEmpty     bool     `json:",default=true"`
	DisableDefaultEndpoint bool     `json:",optional"`
}

// Normalize applies defaults and ETCD_ENDPOINTS/BEEHIVE_ENV fallback.
// Normalize 应用默认值和 ETCD_ENDPOINTS/BEEHIVE_ENV 回退。
func (c Config) Normalize() Config {
	if c.PreferEnvWhenEmpty || len(c.Endpoints) == 0 {
		if env := strings.TrimSpace(os.Getenv("ETCD_ENDPOINTS")); len(c.Endpoints) == 0 && env != "" {
			c.Endpoints = splitCSV(env)
		}
		if c.Env == "" {
			c.Env = strings.TrimSpace(os.Getenv("BEEHIVE_ENV"))
		}
	}
	if len(c.Endpoints) == 0 && !c.DisableDefaultEndpoint {
		c.Endpoints = []string{defaultEndpoint}
	}
	if c.Prefix == "" {
		c.Prefix = "/beehive-im"
	}
	if c.Env == "" {
		c.Env = "dev"
	}
	if c.DialTimeoutSeconds <= 0 {
		c.DialTimeoutSeconds = defaultDialTimeoutSeconds
	}
	if c.LeaseTTLSeconds <= 0 {
		c.LeaseTTLSeconds = defaultLeaseTTLSeconds
	}
	return c
}

// NewClient opens an etcd client.
// NewClient 打开 etcd 客户端。
func NewClient(c Config) (*clientv3.Client, error) {
	normalized := c.Normalize()
	if len(normalized.Endpoints) == 0 {
		return nil, errors.New("etcd endpoints are required")
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   normalized.Endpoints,
		DialTimeout: time.Duration(normalized.DialTimeoutSeconds) * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open etcd client: %w", err)
	}
	return cli, nil
}

// ServiceNode is the JSON value stored in etcd service registry.
// ServiceNode 是写入 etcd 服务注册表的 JSON 值。
type ServiceNode struct {
	SchemaVersion int    `json:"schema_version"`
	InstanceID    string `json:"instance_id"`
	Service       string `json:"service"`
	Address       string `json:"address"`
	Status        string `json:"status"`
	SessionCount  int    `json:"session_count,omitempty"`
	MaxSessions   int    `json:"max_sessions,omitempty"`
	UpdatedAt     string `json:"updated_at"`
}

// ServiceKey returns /{prefix}/{env}/services/{service}/{instanceID}.
// ServiceKey 返回 /{prefix}/{env}/services/{service}/{instanceID}。
func ServiceKey(prefix, env, service, instanceID string) string {
	prefix = "/" + strings.Trim(strings.TrimSpace(prefix), "/")
	env = strings.Trim(strings.TrimSpace(env), "/")
	service = strings.Trim(strings.TrimSpace(service), "/")
	instanceID = strings.Trim(strings.TrimSpace(instanceID), "/")
	return fmt.Sprintf("%s/%s/services/%s/%s", prefix, env, service, instanceID)
}

// ServicePrefix returns /{prefix}/{env}/services/{service}/.
// ServicePrefix 返回 /{prefix}/{env}/services/{service}/。
func ServicePrefix(prefix, env, service string) string {
	prefix = "/" + strings.Trim(strings.TrimSpace(prefix), "/")
	env = strings.Trim(strings.TrimSpace(env), "/")
	service = strings.Trim(strings.TrimSpace(service), "/")
	return fmt.Sprintf("%s/%s/services/%s/", prefix, env, service)
}

// EncodeNode serializes a registry node.
// EncodeNode 序列化服务注册节点。
func EncodeNode(node ServiceNode) (string, error) {
	if node.SchemaVersion == 0 {
		node.SchemaVersion = 1
	}
	if node.UpdatedAt == "" {
		node.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.Marshal(node)
	if err != nil {
		return "", fmt.Errorf("encode service node: %w", err)
	}
	return string(data), nil
}

// DecodeNode parses a registry node.
// DecodeNode 解析服务注册节点。
func DecodeNode(data []byte) (ServiceNode, error) {
	var node ServiceNode
	if err := json.Unmarshal(data, &node); err != nil {
		return ServiceNode{}, fmt.Errorf("decode service node: %w", err)
	}
	return node, nil
}

// Registration keeps one service lease alive until Close is called.
// Registration 维护一个服务注册租约，直到调用 Close。
type Registration struct {
	client  *clientv3.Client
	leaseID clientv3.LeaseID
	key     string
	done    chan struct{}
	cancel  context.CancelFunc
}

// RegisterService writes one service node with a renewable lease.
// RegisterService 写入带续租租约的服务节点。
func RegisterService(ctx context.Context, cli *clientv3.Client, cfg Config, service string, node ServiceNode) (*Registration, error) {
	normalized := cfg.Normalize()
	if cli == nil {
		return nil, errors.New("etcd client is nil")
	}
	lease, err := cli.Grant(ctx, normalized.LeaseTTLSeconds)
	if err != nil {
		return nil, fmt.Errorf("grant etcd lease: %w", err)
	}
	key := ServiceKey(normalized.Prefix, normalized.Env, service, node.InstanceID)
	value, err := EncodeNode(node)
	if err != nil {
		return nil, err
	}
	if _, err := cli.Put(ctx, key, value, clientv3.WithLease(lease.ID)); err != nil {
		_, _ = cli.Revoke(context.Background(), lease.ID)
		return nil, fmt.Errorf("put service node: %w", err)
	}
	keepCtx, cancel := context.WithCancel(context.Background())
	keepAlive, err := cli.KeepAlive(keepCtx, lease.ID)
	if err != nil {
		cancel()
		_, _ = cli.Revoke(context.Background(), lease.ID)
		return nil, fmt.Errorf("keepalive service lease: %w", err)
	}
	reg := &Registration{
		client:  cli,
		leaseID: lease.ID,
		key:     key,
		done:    make(chan struct{}),
		cancel:  cancel,
	}
	go func() {
		for {
			select {
			case _, ok := <-keepAlive:
				if !ok {
					return
				}
			case <-reg.done:
				return
			}
		}
	}()
	return reg, nil
}

// Update refreshes the registered node value.
// Update 刷新已注册节点的 value。
func (r *Registration) Update(ctx context.Context, node ServiceNode) error {
	value, err := EncodeNode(node)
	if err != nil {
		return err
	}
	_, err = r.client.Put(ctx, r.key, value, clientv3.WithLease(r.leaseID))
	if err != nil {
		return fmt.Errorf("update service node: %w", err)
	}
	return nil
}

// Close revokes the service lease.
// Close 撤销服务注册租约。
func (r *Registration) Close(ctx context.Context) error {
	select {
	case <-r.done:
	default:
		close(r.done)
	}
	if r.cancel != nil {
		r.cancel()
	}
	if _, err := r.client.Revoke(ctx, r.leaseID); err != nil {
		return fmt.Errorf("revoke service lease: %w", err)
	}
	return nil
}

// ListService returns all nodes under a service prefix.
// ListService 返回服务前缀下的所有节点。
func ListService(ctx context.Context, cli *clientv3.Client, cfg Config, service string) ([]ServiceNode, error) {
	normalized := cfg.Normalize()
	prefix := ServicePrefix(normalized.Prefix, normalized.Env, service)
	resp, err := cli.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("list service nodes: %w", err)
	}
	nodes := make([]ServiceNode, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		node, err := DecodeNode(kv.Value)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
