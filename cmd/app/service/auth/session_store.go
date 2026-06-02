package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/HappyLadySauce/Beehive-IM/pkg/common/cache"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/jwt"
	"github.com/redis/go-redis/v9"
)

// SessionRecord is the Redis-backed session state used for refresh-token validation.
// SessionRecord 是 Redis 中用于 refresh token 校验的会话状态。
type SessionRecord struct {
	RefreshHash string `json:"refresh_hash"`
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DeviceID    string `json:"device_id"`
	Platform    string `json:"platform"`
}

// SessionIsActive reports whether the session key still exists in Redis (not expired and not revoked).
// SessionIsActive 根据 Redis 中会话键是否仍存在判断会话是否有效（未过期且未吊销）。
func (s *AuthService) SessionIsActive(ctx context.Context, sessionID string) (bool, error) {
	if s == nil || s.Cache == nil {
		return false, fmt.Errorf("auth service is not fully initialized")
	}
	n, err := s.Cache.Exists(ctx, cache.SessionIDPrefix+sessionID).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ValidateRefreshToken checks the refresh token against the hash stored for sessionID.
// ValidateRefreshToken 校验 refresh 是否与 session 键中存储的哈希一致（供刷新接口使用）。
func (s *AuthService) ValidateRefreshToken(ctx context.Context, sessionID, refreshToken string) error {
	record, err := s.getSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if record.RefreshHash != jwt.HashRefreshToken(refreshToken) {
		return fmt.Errorf("invalid refresh token")
	}
	return nil
}

// DeleteSession revokes the session by deleting its Redis key.
// DeleteSession 删除会话 Redis 键，立即使 access / refresh 失效。
func (s *AuthService) DeleteSession(ctx context.Context, sessionID string) error {
	if s == nil || s.Cache == nil {
		return fmt.Errorf("auth service is not fully initialized")
	}
	if err := s.Cache.Del(ctx, cache.SessionIDPrefix+sessionID).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *AuthService) storeSession(ctx context.Context, sessionID, refreshToken string, record SessionRecord) error {
	record.RefreshHash = jwt.HashRefreshToken(refreshToken)
	return s.saveSession(ctx, sessionID, record)
}

func (s *AuthService) getSession(ctx context.Context, sessionID string) (SessionRecord, error) {
	if s == nil || s.Cache == nil {
		return SessionRecord{}, fmt.Errorf("auth service is not fully initialized")
	}
	raw, err := s.Cache.Get(ctx, cache.SessionIDPrefix+sessionID).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return SessionRecord{}, fmt.Errorf("session expired or revoked")
		}
		return SessionRecord{}, fmt.Errorf("get session: %w", err)
	}
	var record SessionRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return SessionRecord{}, fmt.Errorf("decode session: %w", err)
	}
	return record, nil
}

func (s *AuthService) saveSession(ctx context.Context, sessionID string, record SessionRecord) error {
	if s == nil || s.Cache == nil || s.Config == nil || s.Config.JWT == nil {
		return fmt.Errorf("auth service is not fully initialized")
	}
	value, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	ttl := s.Config.JWT.RefreshTTL
	if err := s.Cache.Set(ctx, cache.SessionIDPrefix+sessionID, string(value), ttl).Err(); err != nil {
		return fmt.Errorf("store session in cache: %w", err)
	}
	return nil
}
