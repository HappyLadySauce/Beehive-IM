package svc

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/pkg/authjwt"
	pkgpostgres "github.com/HappyLadySauce/Beehive-IM/pkg/postgres"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/auth/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServiceContext struct {
	Config config.Config
	DB     *pgxpool.Pool
	Auth   *repository.Repository
	JWT    *authjwt.Manager
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool, err := pkgpostgres.NewPool(context.Background(), c.Postgres)
	if err != nil {
		panic(err)
	}
	jwtManager, err := authjwt.NewManager(c.JWT)
	if err != nil {
		pool.Close()
		panic(err)
	}
	return &ServiceContext{
		Config: c,
		DB:     pool,
		Auth:   repository.New(pool),
		JWT:    jwtManager,
	}
}

func (s *ServiceContext) Close() {
	if s.DB != nil {
		s.DB.Close()
	}
}
