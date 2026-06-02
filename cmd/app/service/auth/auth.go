package auth

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/svc"
	"github.com/HappyLadySauce/Beehive-IM/pkg/config"
)

type AuthService struct {
	DB     *gorm.DB
	Cache  *redis.Client
	Config *config.Config
}

func NewAuthService(s *svc.ServiceContext) *AuthService {
	return &AuthService{
		Cache:  s.Cache,
		DB:     s.DB,
		Config: s.Config,
	}
}

