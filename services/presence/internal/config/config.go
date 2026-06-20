package config

import (
	pkgredis "github.com/HappyLadySauce/Beehive-IM/pkg/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Env                string `json:",default=dev"`
	PresenceTTLSeconds int64  `json:",default=90"`
	RedisStore         pkgredis.Config
}
