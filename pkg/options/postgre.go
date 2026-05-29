package options

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/pflag"
)

// PostgreOptions holds CLI / config knobs for PostgreSQL pool settings.
// PostgreOptions 保存 PostgreSQL 连接池相关的 CLI 与配置项。
type PostgreOptions struct {
	Host            string        `json:"host"              mapstructure:"host"`
	Port            int           `json:"port"              mapstructure:"port"`
	User            string        `json:"user"              mapstructure:"user"`
	Password        string        `json:"password"          mapstructure:"password"`
	DB              string        `json:"db"                mapstructure:"db"`
	SSLMode         string        `json:"ssl-mode"          mapstructure:"ssl-mode"`
	MaxIdleConns    int           `json:"max-idle-conns"    mapstructure:"max-idle-conns"`
	MaxOpenConns    int           `json:"max-open-conns"    mapstructure:"max-open-conns"`
	ConnMaxLifetime time.Duration `json:"conn-max-lifetime" mapstructure:"conn-max-lifetime"`
	ConnMaxIdleTime time.Duration `json:"conn-max-idle-time" mapstructure:"conn-max-idle-time"`
}

func NewPostgreOptions() *PostgreOptions {
	return &PostgreOptions{}
}

func (p *PostgreOptions) Validate() error {
	var err error
	if p.Host == "" {
		err = errors.Join(err, fmt.Errorf("host is required"))
	}
	if p.Port == 0 {
		err = errors.Join(err, fmt.Errorf("port is required"))
	} else if p.Port < minPort || p.Port > maxPort {
		err = errors.Join(err, fmt.Errorf("port must be between %d and %d inclusive, got %d", minPort, maxPort, p.Port))
	}
	if p.User == "" {
		err = errors.Join(err, fmt.Errorf("user is required"))
	}
	if p.DB == "" {
		err = errors.Join(err, fmt.Errorf("db is required"))
	}
	if p.SSLMode == "" {
		err = errors.Join(err, fmt.Errorf("ssl-mode is required"))
	} else if !postgreSSLModeKnown(p.SSLMode) {
		err = errors.Join(err, fmt.Errorf("ssl-mode must be one of disable, allow, prefer, require, verify-ca, verify-full, got %q", p.SSLMode))
	}
	if p.MaxIdleConns < 0 {
		err = errors.Join(err, fmt.Errorf("max-idle-conns must be >= 0, got %d", p.MaxIdleConns))
	}
	if p.MaxOpenConns < 0 {
		err = errors.Join(err, fmt.Errorf("max-open-conns must be >= 0, got %d", p.MaxOpenConns))
	}
	if p.MaxOpenConns > 0 && p.MaxIdleConns > p.MaxOpenConns {
		err = errors.Join(err, fmt.Errorf("max-idle-conns must be <= max-open-conns when max-open-conns > 0, got %d and %d", p.MaxIdleConns, p.MaxOpenConns))
	}
	if p.ConnMaxLifetime < 0 {
		err = errors.Join(err, fmt.Errorf("conn-max-lifetime must be >= 0, got %s", p.ConnMaxLifetime))
	}
	if p.ConnMaxIdleTime < 0 {
		err = errors.Join(err, fmt.Errorf("conn-max-idle-time must be >= 0, got %s", p.ConnMaxIdleTime))
	}
	return err
}

func (p *PostgreOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&p.Host, "postgres-host", "127.0.0.1", "PostgreSQL hostname or IP address")
	fs.IntVar(&p.Port, "postgres-port", 5432, "PostgreSQL server TCP port")
	fs.StringVar(&p.User, "postgres-user", "Beehive-Blog", "PostgreSQL user name")
	fs.StringVar(&p.Password, "postgres-password", "", "PostgreSQL password")
	fs.StringVar(&p.DB, "postgres-db", "Beehive-Blog", "PostgreSQL database name")
	fs.StringVar(&p.SSLMode, "postgres-ssl-mode", "disable", "PostgreSQL sslmode (disable, allow, prefer, require, verify-ca, verify-full)")
	fs.IntVar(&p.MaxIdleConns, "postgres-max-idle-conns", 10, "Maximum number of idle connections in the pool")
	fs.IntVar(&p.MaxOpenConns, "postgres-max-open-conns", 100, "Maximum number of open connections (0 means unlimited)")
	fs.DurationVar(&p.ConnMaxLifetime, "postgres-conn-max-lifetime", time.Hour, "Maximum time a connection may be reused (0 means no limit)")
	fs.DurationVar(&p.ConnMaxIdleTime, "postgres-conn-max-idle-time", 30*time.Minute, "Maximum time a connection may be idle before closing (0 means no limit)")
}

// postgreSSLModeKnown reports whether v is a libpq-compatible sslmode value.
// postgreSSLModeKnown 判断 v 是否为 libpq 兼容的 sslmode 取值。
func postgreSSLModeKnown(v string) bool {
	switch v {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		return true
	default:
		return false
	}
}
