package options

import (
	"errors"
	"fmt"
	"math"

	"github.com/spf13/pflag"
)

// Redis logical database index bounds (0-based; server still enforces CONFIG databases).
// Redis 逻辑库下标范围（从 0 开始；具体可用数量由服务端 databases 配置决定）。
const (
	redisMinDB = 0
	redisMaxDB = math.MaxInt32
)

// RedisOptions holds CLI / config knobs for a standalone Redis client.
// RedisOptions 保存独立 Redis 客户端所需的 CLI 与配置项。
type RedisOptions struct {
	Host     string `json:"host"     mapstructure:"host"`
	Port     int    `json:"port"     mapstructure:"port"`
	Password string `json:"password" mapstructure:"password"`
	DB       int    `json:"db"       mapstructure:"db"`
}

func NewRedisOptions() *RedisOptions {
	return &RedisOptions{}
}

func (r *RedisOptions) Validate() error {
	var err error
	if r.Host == "" {
		err = errors.Join(err, fmt.Errorf("host is required"))
	}
	if r.Port == 0 {
		err = errors.Join(err, fmt.Errorf("port is required"))
	} else if r.Port < minPort || r.Port > maxPort {
		err = errors.Join(err, fmt.Errorf("port must be between %d and %d inclusive, got %d", minPort, maxPort, r.Port))
	}
	if r.DB < redisMinDB || r.DB > redisMaxDB {
		err = errors.Join(err, fmt.Errorf("db must be between %d and %d inclusive, got %d", redisMinDB, redisMaxDB, r.DB))
	}
	return err
}

func (r *RedisOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&r.Host, "redis-host", "127.0.0.1", "Redis hostname or IP address")
	fs.IntVar(&r.Port, "redis-port", 6379, "Redis server TCP port")
	fs.StringVar(&r.Password, "redis-password", "", "Redis password (empty when the server has no auth)")
	fs.IntVar(&r.DB, "redis-db", 0, "Redis logical database index (SELECT)")
}
