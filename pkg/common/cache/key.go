package cache

const (
	// SessionIDPrefix is the only session key; value is refresh token hash; TTL = refresh lifetime.
	// SessionIDPrefix 为唯一会话键，值为 refresh 哈希，TTL 为 refresh 生命周期。
	SessionIDPrefix = "auth:session:"

	// PresenceUserInstancesPrefix stores the online instance set for one user.
	// PresenceUserInstancesPrefix 保存单个用户当前在线实例集合。
	PresenceUserInstancesPrefix = "im:presence:user:"

	// PresenceSessionPrefix stores one websocket session's routing metadata.
	// PresenceSessionPrefix 保存单个 WebSocket 会话的路由元数据。
	PresenceSessionPrefix = "im:presence:session:"
)
