package config

import (
	"github.com/HappyLadySauce/Beehive-IM/pkg/postgres"
	pkgredis "github.com/HappyLadySauce/Beehive-IM/pkg/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Env        string `json:",default=dev"`
	Postgres   postgres.Config
	RedisStore pkgredis.Config
}
