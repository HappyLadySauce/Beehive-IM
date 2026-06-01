package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/HappyLadySauce/Beehive-IM/pkg/common/cache"
)

type AuthService struct {
	Cache  *redis.Client
}

func NewAuthService(cache *redis.Client) *AuthService {
	return &AuthService{
		Cache: cache,
	}
}

func (s *AuthService) SetUserTokenVersion(ctx context.Context, userID, version string) error {
	key := cache.UserTokenVersionPrefix + userID

	return s.Cache.Set(ctx, key, version, time.Duration(time.Hour)).Err()
}

func (s *AuthService) GetUserTokenVersion(ctx context.Context, userID string) (string, error) {
	key := cache.UserTokenVersionPrefix + userID

	val, err := s.Cache.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}

	return val, nil
}

func (s *AuthService) DeleteUserTokenVersion(ctx context.Context, userID string) error {
	key := cache.UserTokenVersionPrefix + userID

	return s.Cache.Del(ctx, key).Err()
}

