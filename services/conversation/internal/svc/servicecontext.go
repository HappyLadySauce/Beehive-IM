package svc

import (
	"context"

	pkgpostgres "github.com/HappyLadySauce/Beehive-IM/pkg/postgres"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/user/userservice"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config        config.Config
	DB            *pgxpool.Pool
	Conversations *repository.Repository
	User          userservice.UserService
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool, err := pkgpostgres.NewPool(context.Background(), c.Postgres)
	if err != nil {
		panic(err)
	}
	return &ServiceContext{
		Config:        c,
		DB:            pool,
		Conversations: repository.New(pool),
		User:          userservice.NewUserService(zrpc.MustNewClient(c.User)),
	}
}

func (s *ServiceContext) Close() {
	if s.DB != nil {
		s.DB.Close()
	}
}
