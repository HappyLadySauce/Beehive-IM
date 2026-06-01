package user

import (

	"gorm.io/gorm"
	"github.com/redis/go-redis/v9"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
)

type UserService struct {
	DB  *gorm.DB
	Cache *redis.Client
}

func NewUserService(svc *svc.ServiceContext) *UserService {
	return &UserService{
		DB:  svc.DB,
		Cache: svc.Cache,
	}
}
