package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultDSN                   = "postgres://Beehive-IM:Beehive-IM@127.0.0.1:5432/Beehive-IM?sslmode=disable"
	defaultConnectTimeoutSeconds = 5
	defaultMaxConns              = int32(20)
	defaultMinConns              = int32(2)
)

// Config describes PostgreSQL connection pool settings.
// Config 描述 PostgreSQL 连接池配置。
type Config struct {
	DSN                      string `json:",optional"`
	MaxConns                 int32  `json:",default=20"`
	MinConns                 int32  `json:",default=2"`
	MaxConnLifetimeSeconds   int64  `json:",default=1800"`
	MaxConnIdleTimeSeconds   int64  `json:",default=300"`
	ConnectTimeoutSeconds    int64  `json:",default=5"`
	HealthCheckPeriodSeconds int64  `json:",default=30"`
	PreferEnvWhenEmpty       bool   `json:",default=true"`
	DisableDefaultLocalDSN   bool   `json:",optional"`
}

// Normalize applies defaults and DB_DSN fallback.
// Normalize 应用默认值和 DB_DSN 回退。
func (c Config) Normalize() Config {
	if c.PreferEnvWhenEmpty || c.DSN == "" {
		if env := strings.TrimSpace(os.Getenv("DB_DSN")); c.DSN == "" && env != "" {
			c.DSN = env
		}
	}
	if c.DSN == "" && !c.DisableDefaultLocalDSN {
		c.DSN = defaultDSN
	}
	if c.MaxConns <= 0 {
		c.MaxConns = defaultMaxConns
	}
	if c.MinConns < 0 {
		c.MinConns = defaultMinConns
	}
	if c.MinConns > c.MaxConns {
		c.MinConns = c.MaxConns
	}
	if c.ConnectTimeoutSeconds <= 0 {
		c.ConnectTimeoutSeconds = defaultConnectTimeoutSeconds
	}
	if c.HealthCheckPeriodSeconds <= 0 {
		c.HealthCheckPeriodSeconds = 30
	}
	return c
}

// PoolConfig builds a pgxpool configuration without opening connections.
// PoolConfig 构建 pgxpool 配置但不打开连接。
func PoolConfig(c Config) (*pgxpool.Config, error) {
	normalized := c.Normalize()
	if strings.TrimSpace(normalized.DSN) == "" {
		return nil, errors.New("postgres dsn is required")
	}

	pgxCfg, err := pgxpool.ParseConfig(normalized.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	pgxCfg.MaxConns = normalized.MaxConns
	pgxCfg.MinConns = normalized.MinConns
	if normalized.MaxConnLifetimeSeconds > 0 {
		pgxCfg.MaxConnLifetime = time.Duration(normalized.MaxConnLifetimeSeconds) * time.Second
	}
	if normalized.MaxConnIdleTimeSeconds > 0 {
		pgxCfg.MaxConnIdleTime = time.Duration(normalized.MaxConnIdleTimeSeconds) * time.Second
	}
	pgxCfg.HealthCheckPeriod = time.Duration(normalized.HealthCheckPeriodSeconds) * time.Second
	return pgxCfg, nil
}

// NewPool opens a PostgreSQL pool and verifies connectivity.
// NewPool 打开 PostgreSQL 连接池并验证连通性。
func NewPool(ctx context.Context, c Config) (*pgxpool.Pool, error) {
	normalized := c.Normalize()
	pgxCfg, err := PoolConfig(normalized)
	if err != nil {
		return nil, err
	}

	connectCtx, cancel := context.WithTimeout(ctx, time.Duration(normalized.ConnectTimeoutSeconds)*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, pgxCfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}
