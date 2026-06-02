package user

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
	"github.com/HappyLadySauce/Beehive-IM/pkg/config"
)

type UserService struct {
	DB     *gorm.DB
	Cache  *redis.Client
	Config *config.Config
}

func NewUserService(svc *svc.ServiceContext) *UserService {
	return &UserService{
		DB:     svc.DB,
		Cache:  svc.Cache,
		Config: svc.Config,
	}
}
