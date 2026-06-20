package postgres

import (
	"os"
	"testing"
)

func TestNormalizeUsesEnvDSN(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://user:pass@127.0.0.1:5432/db?sslmode=disable")
	cfg := Config{DisableDefaultLocalDSN: true}.Normalize()
	if cfg.DSN != os.Getenv("DB_DSN") {
		t.Fatalf("DSN = %q, want env DSN", cfg.DSN)
	}
}

func TestPoolConfigRejectsInvalidDSN(t *testing.T) {
	_, err := PoolConfig(Config{DSN: "://bad", DisableDefaultLocalDSN: true})
	if err == nil {
		t.Fatal("PoolConfig() error = nil, want error")
	}
}

func TestPoolConfigAppliesPoolDefaults(t *testing.T) {
	cfg, err := PoolConfig(Config{DSN: "postgres://user:pass@127.0.0.1:5432/db?sslmode=disable"})
	if err != nil {
		t.Fatalf("PoolConfig() error = %v", err)
	}
	if cfg.MaxConns != defaultMaxConns {
		t.Fatalf("MaxConns = %d, want %d", cfg.MaxConns, defaultMaxConns)
	}
}
