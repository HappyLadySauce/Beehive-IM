package redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	defaultAddr           = "127.0.0.1:6379"
	defaultPoolSize       = 20
	defaultTimeoutSeconds = 3
)

// Config describes Redis client settings.
// Config 描述 Redis 客户端配置。
type Config struct {
	Addr                   string `json:",optional"`
	Password               string `json:",optional"`
	DB                     int    `json:",default=0"`
	PoolSize               int    `json:",default=20"`
	DialTimeoutSeconds     int    `json:",default=3"`
	ReadTimeoutSeconds     int    `json:",default=3"`
	WriteTimeoutSeconds    int    `json:",default=3"`
	PingTimeoutSeconds     int    `json:",default=3"`
	PreferEnvWhenEmpty     bool   `json:",default=true"`
	DisableDefaultLocalURL bool   `json:",optional"`
}

// Normalize applies defaults and REDIS_ADDR/REDIS_PASSWORD/REDIS_DB fallback.
// Normalize 应用默认值和 REDIS_ADDR/REDIS_PASSWORD/REDIS_DB 回退。
func (c Config) Normalize() Config {
	if c.PreferEnvWhenEmpty || c.Addr == "" {
		if env := strings.TrimSpace(os.Getenv("REDIS_ADDR")); c.Addr == "" && env != "" {
			c.Addr = env
		}
		if c.Password == "" {
			c.Password = os.Getenv("REDIS_PASSWORD")
		}
		if db := strings.TrimSpace(os.Getenv("REDIS_DB")); db != "" && c.DB == 0 {
			if parsed, err := strconv.Atoi(db); err == nil {
				c.DB = parsed
			}
		}
	}
	if c.Addr == "" && !c.DisableDefaultLocalURL {
		c.Addr = defaultAddr
	}
	if c.PoolSize <= 0 {
		c.PoolSize = defaultPoolSize
	}
	if c.DialTimeoutSeconds <= 0 {
		c.DialTimeoutSeconds = defaultTimeoutSeconds
	}
	if c.ReadTimeoutSeconds <= 0 {
		c.ReadTimeoutSeconds = defaultTimeoutSeconds
	}
	if c.WriteTimeoutSeconds <= 0 {
		c.WriteTimeoutSeconds = defaultTimeoutSeconds
	}
	if c.PingTimeoutSeconds <= 0 {
		c.PingTimeoutSeconds = defaultTimeoutSeconds
	}
	return c
}

// Options builds go-redis options without opening a connection.
// Options 构建 go-redis 配置但不打开连接。
func Options(c Config) (*goredis.Options, error) {
	normalized := c.Normalize()
	if strings.TrimSpace(normalized.Addr) == "" {
		return nil, errors.New("redis addr is required")
	}
	return &goredis.Options{
		Addr:         normalized.Addr,
		Password:     normalized.Password,
		DB:           normalized.DB,
		PoolSize:     normalized.PoolSize,
		DialTimeout:  time.Duration(normalized.DialTimeoutSeconds) * time.Second,
		ReadTimeout:  time.Duration(normalized.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(normalized.WriteTimeoutSeconds) * time.Second,
	}, nil
}

// NewClient opens a Redis client and verifies connectivity.
// NewClient 打开 Redis 客户端并验证连通性。
func NewClient(ctx context.Context, c Config) (*goredis.Client, error) {
	normalized := c.Normalize()
	opts, err := Options(normalized)
	if err != nil {
		return nil, err
	}
	client := goredis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, time.Duration(normalized.PingTimeoutSeconds)*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}
