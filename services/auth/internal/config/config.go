package config

import (
	"github.com/HappyLadySauce/Beehive-IM/pkg/authjwt"
	"github.com/HappyLadySauce/Beehive-IM/pkg/postgres"
	pkgredis "github.com/HappyLadySauce/Beehive-IM/pkg/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Env        string `json:",default=dev"`
	Postgres   postgres.Config
	RedisStore pkgredis.Config
	JWT        authjwt.Config

	// RefreshTokenTTLSeconds controls long-lived refresh token lifetime.
	// RefreshTokenTTLSeconds 控制长期刷新令牌有效期。
	RefreshTokenTTLSeconds int64 `json:",default=2592000"`
}
