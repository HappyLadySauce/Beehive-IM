package cache

const (
	// SessionIDPrefix is the only session key; value is refresh token hash; TTL = refresh lifetime.
	// SessionIDPrefix 为唯一会话键，值为 refresh 哈希，TTL 为 refresh 生命周期。
	SessionIDPrefix = "auth:session:"
)
