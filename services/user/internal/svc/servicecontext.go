package svc

import (
	"context"

	pkgpostgres "github.com/HappyLadySauce/Beehive-IM/pkg/postgres"
	"github.com/HappyLadySauce/Beehive-IM/services/user/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/user/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServiceContext struct {
	Config config.Config
	DB     *pgxpool.Pool
	Users  *repository.UserRepository
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool, err := pkgpostgres.NewPool(context.Background(), c.Postgres)
	if err != nil {
		panic(err)
	}
	return &ServiceContext{
		Config: c,
		DB:     pool,
		Users:  repository.NewUserRepository(pool),
	}
}

func (s *ServiceContext) Close() {
	if s.DB != nil {
		s.DB.Close()
	}
}
