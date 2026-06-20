package presence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/presence/presenceservice"
)

// ConnectionMeta carries Edge-owned connection state for Presence.
// ConnectionMeta 携带 Edge 拥有的连接状态。
type ConnectionMeta struct {
	SessionID string
	ConnID    string
	EdgeID    string
	UserID    string
	DeviceID  string
	GatewayID string
}

// Client defines the Presence boundary used by Edge.
// Client 定义 Edge 使用的 Presence 边界。
type Client interface {
	UpsertConnection(ctx context.Context, conn ConnectionMeta) error
	RefreshConnection(ctx context.Context, conn ConnectionMeta) error
	RebindGateway(ctx context.Context, conn ConnectionMeta) error
	RemoveConnection(ctx context.Context, conn ConnectionMeta) error
}

// NoopClient keeps the MVP independent from the real Presence service.
// NoopClient 让 MVP 不依赖真实 Presence 服务。
type NoopClient struct{}

func NewNoopClient() NoopClient {
	return NoopClient{}
}

func (NoopClient) UpsertConnection(ctx context.Context, conn ConnectionMeta) error {
	return nil
}

func (NoopClient) RefreshConnection(ctx context.Context, conn ConnectionMeta) error {
	return nil
}

func (NoopClient) RebindGateway(ctx context.Context, conn ConnectionMeta) error {
	return nil
}

func (NoopClient) RemoveConnection(ctx context.Context, conn ConnectionMeta) error {
	return nil
}

// RPCClient calls the Presence zRPC service.
// RPCClient 调用 Presence zRPC 服务。
type RPCClient struct {
	client     presenceservice.PresenceService
	ttlSeconds int64
	timeout    time.Duration
}

func NewRPCClient(client presenceservice.PresenceService, ttlSeconds int64) *RPCClient {
	if ttlSeconds <= 0 {
		ttlSeconds = 90
	}
	return &RPCClient{
		client:     client,
		ttlSeconds: ttlSeconds,
		timeout:    2 * time.Second,
	}
}

func (c *RPCClient) UpsertConnection(ctx context.Context, conn ConnectionMeta) error {
	if c == nil || c.client == nil {
		return errors.New("presence client is unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.UpsertConnection(callCtx, &presenceservice.UpsertConnectionRequest{
		Connection: &presenceservice.ConnectionMeta{
			SessionId: conn.SessionID,
			ConnId:    conn.ConnID,
			EdgeId:    conn.EdgeID,
			UserId:    conn.UserID,
			DeviceId:  conn.DeviceID,
			GatewayId: conn.GatewayID,
		},
		TtlSeconds: c.ttlSeconds,
	})
	if err != nil {
		return fmt.Errorf("presence upsert rpc: %w", err)
	}
	if !resp.GetAccepted() {
		return fmt.Errorf("presence upsert rejected: %s %s", resp.GetErrorCode(), resp.GetMessage())
	}
	return nil
}

func (c *RPCClient) RefreshConnection(ctx context.Context, conn ConnectionMeta) error {
	if c == nil || c.client == nil {
		return errors.New("presence client is unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.RefreshConnection(callCtx, &presenceservice.RefreshConnectionRequest{
		SessionId:  conn.SessionID,
		ConnId:     conn.ConnID,
		EdgeId:     conn.EdgeID,
		TtlSeconds: c.ttlSeconds,
	})
	if err != nil {
		return fmt.Errorf("presence refresh rpc: %w", err)
	}
	if !resp.GetRefreshed() {
		return fmt.Errorf("presence refresh rejected: %s %s", resp.GetErrorCode(), resp.GetMessage())
	}
	return nil
}

func (c *RPCClient) RebindGateway(ctx context.Context, conn ConnectionMeta) error {
	if c == nil || c.client == nil {
		return errors.New("presence client is unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.RebindGateway(callCtx, &presenceservice.RebindGatewayRequest{
		SessionId:  conn.SessionID,
		ConnId:     conn.ConnID,
		EdgeId:     conn.EdgeID,
		GatewayId:  conn.GatewayID,
		TtlSeconds: c.ttlSeconds,
	})
	if err != nil {
		return fmt.Errorf("presence rebind rpc: %w", err)
	}
	if !resp.GetRebound() {
		return fmt.Errorf("presence rebind rejected: %s %s", resp.GetErrorCode(), resp.GetMessage())
	}
	return nil
}

func (c *RPCClient) RemoveConnection(ctx context.Context, conn ConnectionMeta) error {
	if c == nil || c.client == nil {
		return errors.New("presence client is unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err := c.client.RemoveConnection(callCtx, &presenceservice.RemoveConnectionRequest{
		Connection: &presenceservice.ConnectionMeta{
			SessionId: conn.SessionID,
			ConnId:    conn.ConnID,
			EdgeId:    conn.EdgeID,
			UserId:    conn.UserID,
			DeviceId:  conn.DeviceID,
			GatewayId: conn.GatewayID,
		},
	})
	if err != nil {
		return fmt.Errorf("presence remove rpc: %w", err)
	}
	return nil
}
