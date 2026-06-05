package presence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/HappyLadySauce/Beehive-IM/pkg/common/cache"
)

// Session describes one websocket session registered on this instance.
// Session 描述注册在当前实例上的一个 WebSocket 会话。
type Session struct {
	UserID     string
	SessionID  string
	DeviceID   string
	Platform   string
	InstanceID string
}

// Service stores websocket presence in Redis for cross-instance routing.
// Service 将 WebSocket 在线状态存入 Redis，用于跨实例路由。
type Service struct {
	cache      *redis.Client
	instanceID string
	ttl        time.Duration
}

// NewService creates a Redis-backed presence service.
// NewService 创建基于 Redis 的在线状态服务。
func NewService(cacheClient *redis.Client, instanceID string, ttl time.Duration) (*Service, error) {
	if cacheClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return nil, fmt.Errorf("instance id is required")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("presence ttl must be > 0")
	}
	return &Service{
		cache:      cacheClient,
		instanceID: instanceID,
		ttl:        ttl,
	}, nil
}

// InstanceID returns the current application instance id.
// InstanceID 返回当前应用实例 ID。
func (s *Service) InstanceID() string {
	if s == nil {
		return ""
	}
	return s.instanceID
}

// Register records a websocket session and its owning instance.
// Register 记录 WebSocket 会话及其所属实例。
func (s *Service) Register(ctx context.Context, session Session) error {
	if err := s.validateSession(session); err != nil {
		return err
	}
	session.InstanceID = s.instanceID

	userInstancesKey := UserInstancesKey(session.UserID)
	userInstanceSessionsKey := UserInstanceSessionsKey(session.UserID, s.instanceID)
	sessionKey := SessionKey(session.SessionID)

	pipe := s.cache.TxPipeline()
	pipe.SAdd(ctx, userInstancesKey, s.instanceID)
	pipe.Expire(ctx, userInstancesKey, s.ttl)
	pipe.SAdd(ctx, userInstanceSessionsKey, session.SessionID)
	pipe.Expire(ctx, userInstanceSessionsKey, s.ttl)
	pipe.HSet(ctx, sessionKey, map[string]any{
		"user_id":     session.UserID,
		"session_id":  session.SessionID,
		"device_id":   session.DeviceID,
		"platform":    session.Platform,
		"instance_id": s.instanceID,
	})
	pipe.Expire(ctx, sessionKey, s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("register presence: %w", err)
	}
	return nil
}

// Unregister removes one websocket session from Redis presence.
// Unregister 从 Redis 在线状态中移除一个 WebSocket 会话。
func (s *Service) Unregister(ctx context.Context, session Session) error {
	if s == nil || s.cache == nil {
		return fmt.Errorf("presence service is not configured")
	}
	if session.UserID == "" || session.SessionID == "" {
		return fmt.Errorf("presence session is incomplete")
	}

	userInstancesKey := UserInstancesKey(session.UserID)
	userInstanceSessionsKey := UserInstanceSessionsKey(session.UserID, s.instanceID)
	sessionKey := SessionKey(session.SessionID)

	pipe := s.cache.TxPipeline()
	pipe.Del(ctx, sessionKey)
	pipe.SRem(ctx, userInstanceSessionsKey, session.SessionID)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("unregister presence: %w", err)
	}

	count, err := s.cache.SCard(ctx, userInstanceSessionsKey).Result()
	if err != nil {
		return fmt.Errorf("count instance sessions: %w", err)
	}
	if count == 0 {
		pipe = s.cache.TxPipeline()
		pipe.Del(ctx, userInstanceSessionsKey)
		pipe.SRem(ctx, userInstancesKey, s.instanceID)
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("cleanup instance presence: %w", err)
		}
	}
	return nil
}

// Refresh renews TTLs for a local websocket session.
// Refresh 刷新本地 WebSocket 会话相关 Redis key 的 TTL。
func (s *Service) Refresh(ctx context.Context, session Session) error {
	if err := s.validateSession(session); err != nil {
		return err
	}
	pipe := s.cache.TxPipeline()
	pipe.Expire(ctx, UserInstancesKey(session.UserID), s.ttl)
	pipe.Expire(ctx, UserInstanceSessionsKey(session.UserID, s.instanceID), s.ttl)
	pipe.Expire(ctx, SessionKey(session.SessionID), s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("refresh presence: %w", err)
	}
	return nil
}

// InstancesForUser returns the instance ids that currently host the user.
// InstancesForUser 返回当前承载该用户连接的实例 ID 集合。
func (s *Service) InstancesForUser(ctx context.Context, userID string) ([]string, error) {
	if s == nil || s.cache == nil {
		return nil, fmt.Errorf("presence service is not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	instances, err := s.cache.SMembers(ctx, UserInstancesKey(userID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user instances: %w", err)
	}
	return instances, nil
}

func (s *Service) validateSession(session Session) error {
	if s == nil || s.cache == nil {
		return fmt.Errorf("presence service is not configured")
	}
	if strings.TrimSpace(session.UserID) == "" {
		return fmt.Errorf("user id is required")
	}
	if strings.TrimSpace(session.SessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	return nil
}

// UserInstancesKey returns the Redis set key for one user's online instances.
// UserInstancesKey 返回单个用户在线实例集合的 Redis key。
func UserInstancesKey(userID string) string {
	return cache.PresenceUserInstancesPrefix + userID + ":instances"
}

// UserInstanceSessionsKey returns the Redis set key for sessions on one instance.
// UserInstanceSessionsKey 返回用户在单个实例上的 session 集合 Redis key。
func UserInstanceSessionsKey(userID, instanceID string) string {
	return cache.PresenceUserInstancesPrefix + userID + ":instance:" + instanceID + ":sessions"
}

// SessionKey returns the Redis hash key for one websocket session.
// SessionKey 返回单个 WebSocket 会话的 Redis hash key。
func SessionKey(sessionID string) string {
	return cache.PresenceSessionPrefix + sessionID
}
