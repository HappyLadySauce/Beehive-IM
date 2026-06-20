package presence

import "context"

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

func (NoopClient) RemoveConnection(ctx context.Context, conn ConnectionMeta) error {
	return nil
}
