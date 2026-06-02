package auth

import (
	"context"
	"fmt"

	"github.com/HappyLadySauce/Beehive-IM/pkg/common/cache"
	"github.com/HappyLadySauce/Beehive-IM/pkg/utils/jwt"
	"github.com/redis/go-redis/v9"
)

// GetSessionVersion returns the live session version stored in Redis.
// GetSessionVersion 返回 Redis 中当前会话版本。
func (s *AuthService) GetSessionVersion(ctx context.Context, sessionID string) (string, error) {
	if s == nil || s.Cache == nil {
		return "", fmt.Errorf("auth service is not fully initialized")
	}
	return s.Cache.Get(ctx, cache.SessionPrefix+sessionID).Result()
}

// DeleteSession revokes access and refresh credentials for the session.
// DeleteSession 吊销该会话的访问与刷新凭证。
func (s *AuthService) DeleteSession(ctx context.Context, sessionID string) error {
	if s == nil || s.Cache == nil {
		return fmt.Errorf("auth service is not fully initialized")
	}

	refreshHash, err := s.Cache.Get(ctx, cache.SessionRefreshPrefix+sessionID).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("get session refresh mapping: %w", err)
	}

	keys := []string{cache.SessionPrefix + sessionID, cache.SessionRefreshPrefix + sessionID}
	if refreshHash != "" {
		keys = append(keys, cache.RefreshPrefix+refreshHash)
	}
	if err := s.Cache.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("delete session keys: %w", err)
	}
	return nil
}

func (s *AuthService) storeSession(ctx context.Context, sessionID, version, refreshToken string) error {
	refreshHash := jwt.HashRefreshToken(refreshToken)
	ttl := s.Config.JWT.RefreshTTL

	pipe := s.Cache.Pipeline()
	pipe.Set(ctx, cache.SessionPrefix+sessionID, version, ttl)
	pipe.Set(ctx, cache.RefreshPrefix+refreshHash, sessionID, ttl)
	pipe.Set(ctx, cache.SessionRefreshPrefix+sessionID, refreshHash, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store session in cache: %w", err)
	}
	return nil
}
