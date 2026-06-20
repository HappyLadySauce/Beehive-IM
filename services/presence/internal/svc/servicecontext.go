package svc

import (
	"context"
	"time"

	pkgredis "github.com/HappyLadySauce/Beehive-IM/pkg/redis"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/internal/store"
	goredis "github.com/redis/go-redis/v9"
)

type ServiceContext struct {
	Config config.Config
	Redis  *goredis.Client
	Store  *store.Store
}

func NewServiceContext(c config.Config) *ServiceContext {
	client, err := pkgredis.NewClient(context.Background(), c.RedisStore)
	if err != nil {
		panic(err)
	}
	ttl := time.Duration(c.PresenceTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 90 * time.Second
	}
	return &ServiceContext{
		Config: c,
		Redis:  client,
		Store:  store.New(client, ttl),
	}
}

func (s *ServiceContext) Close() {
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
}
