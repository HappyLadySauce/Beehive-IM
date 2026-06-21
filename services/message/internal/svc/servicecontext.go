package svc

import (
	"context"
	"time"

	pkgpostgres "github.com/HappyLadySauce/Beehive-IM/pkg/postgres"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/conversationservice"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/outbox"
	"github.com/HappyLadySauce/Beehive-IM/services/message/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config       config.Config
	DB           *pgxpool.Pool
	Messages     *repository.Repository
	Conversation conversationservice.ConversationService
	Dispatcher   *outbox.Dispatcher
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool, err := pkgpostgres.NewPool(context.Background(), c.Postgres)
	if err != nil {
		panic(err)
	}
	repo := repository.New(pool)
	conversation := conversationservice.NewConversationService(zrpc.MustNewClient(c.Conversation))
	ctx := &ServiceContext{
		Config:       c,
		DB:           pool,
		Messages:     repo,
		Conversation: conversation,
	}
	if c.Outbox.Enabled {
		dispatcher := outbox.NewDispatcher(outbox.Config{
			BatchSize:      c.Outbox.BatchSize,
			PollInterval:   durationFromMs(c.Outbox.PollIntervalMs, 500*time.Millisecond),
			LockTTL:        time.Duration(defaultInt64(c.Outbox.LockTTLSeconds, 30)) * time.Second,
			MaxAttempts:    defaultInt(c.Outbox.MaxAttempts, 20),
			RetryBaseDelay: durationFromMs(c.Outbox.RetryBaseDelayMs, 500*time.Millisecond),
			RetryMaxDelay:  durationFromMs(c.Outbox.RetryMaxDelayMs, 30*time.Second),
			PublishTimeout: durationFromMs(c.Outbox.PublishTimeoutMs, 5*time.Second),
			RabbitMQ:       c.RabbitMQ,
		}, repo)
		dispatcher.Start(context.Background())
		ctx.Dispatcher = dispatcher
	}
	return ctx
}

func durationFromMs(value int64, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return time.Duration(value) * time.Millisecond
}

func defaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func defaultInt64(value, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func (s *ServiceContext) Close() {
	if s.Dispatcher != nil {
		s.Dispatcher.Stop()
	}
	if s.DB != nil {
		s.DB.Close()
	}
}
