package svc

import (
	"context"
	"time"

	pkgpostgres "github.com/HappyLadySauce/Beehive-IM/pkg/postgres"
	pkgredis "github.com/HappyLadySauce/Beehive-IM/pkg/redis"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/conversationservice"
	"github.com/HappyLadySauce/Beehive-IM/services/notification/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/notification/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/notification/internal/worker"
	"github.com/HappyLadySauce/Beehive-IM/services/presence/presenceservice"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config       config.Config
	DB           *pgxpool.Pool
	Redis        *goredis.Client
	Deliveries   *repository.Repository
	Conversation conversationservice.ConversationService
	Presence     presenceservice.PresenceService
	Consumer     *worker.Consumer
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool, err := pkgpostgres.NewPool(context.Background(), c.Postgres)
	if err != nil {
		panic(err)
	}
	redisClient, err := pkgredis.NewClient(context.Background(), c.Redis)
	if err != nil {
		pool.Close()
		panic(err)
	}
	deliveries := repository.New(pool)
	conversation := conversationservice.NewConversationService(zrpc.MustNewClient(c.Conversation))
	presence := presenceservice.NewPresenceService(zrpc.MustNewClient(c.Presence))
	ctx := &ServiceContext{
		Config:       c,
		DB:           pool,
		Redis:        redisClient,
		Deliveries:   deliveries,
		Conversation: conversation,
		Presence:     presence,
	}
	if c.Worker.Enabled {
		consumer := worker.NewConsumer(worker.Config{
			RabbitMQ:       c.RabbitMQ,
			Queue:          c.Worker.Queue,
			BindingKey:     c.Worker.BindingKey,
			PushExchange:   c.Worker.PushExchange,
			WorkerCount:    c.Worker.WorkerCount,
			DedupeTTL:      durationFromSeconds(c.Worker.DedupeTTLSeconds, 24*time.Hour),
			PublishTimeout: durationFromSeconds(c.Worker.PublishTimeoutSeconds, 5*time.Second),
		}, worker.Dependencies{
			Redis:        redisClient,
			Deliveries:   deliveries,
			Conversation: conversation,
			Presence:     presence,
		})
		consumer.Start(context.Background())
		ctx.Consumer = consumer
	}
	return ctx
}

func durationFromSeconds(value int64, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return time.Duration(value) * time.Second
}

func (s *ServiceContext) Close() {
	if s.Consumer != nil {
		s.Consumer.Stop()
	}
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
	if s.DB != nil {
		s.DB.Close()
	}
}
