package options

import (
	"math"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// TestRedisOptionsValidate exercises RedisOptions.Validate for host, port range, and DB bounds.
// TestRedisOptionsValidate 覆盖主机、端口范围与 DB 下界的 Validate 行为。
func TestRedisOptionsValidate(t *testing.T) {
	tests := []struct {
		name       string
		opts       RedisOptions
		wantNil    bool
		wantSubstr []string
	}{
		{
			name:    "valid_minimal",
			opts:    RedisOptions{Host: "127.0.0.1", Port: 6379, DB: 0},
			wantNil: true,
		},
		{
			name:    "db_zero_ok",
			opts:    RedisOptions{Host: "x", Port: 6379, DB: 0},
			wantNil: true,
		},
		{
			name:    "missing_host_and_port",
			opts:    RedisOptions{},
			wantNil: false,
			wantSubstr: []string{
				"host is required",
				"port is required",
			},
		},
		{
			name:       "port_below_min",
			opts:       RedisOptions{Host: "h", Port: -1, DB: 0},
			wantNil:    false,
			wantSubstr: []string{"port must be between 1 and 65535 inclusive, got -1"},
		},
		{
			name:       "port_above_max",
			opts:       RedisOptions{Host: "h", Port: 65536, DB: 0},
			wantNil:    false,
			wantSubstr: []string{"port must be between 1 and 65535 inclusive, got 65536"},
		},
		{
			name:       "negative_db",
			opts:       RedisOptions{Host: "h", Port: 6379, DB: -1},
			wantNil:    false,
			wantSubstr: []string{"db must be between 0 and 2147483647 inclusive, got -1"},
		},
		{
			name:    "db_max_int32_ok",
			opts:    RedisOptions{Host: "h", Port: 6379, DB: math.MaxInt32},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantNil {
				if err != nil {
					t.Errorf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Errorf("Validate() = nil, want non-nil validation error")
				return
			}
			msg := err.Error()
			for _, s := range tt.wantSubstr {
				if !strings.Contains(msg, s) {
					t.Errorf("Validate() error = %q, want substring %q", msg, s)
				}
			}
		})
	}
}

// TestRedisOptionsAddFlagsDefaults verifies default flag values after Parse(nil).
// TestRedisOptionsAddFlagsDefaults 校验 Parse(nil) 后的默认标志值。
func TestRedisOptionsAddFlagsDefaults(t *testing.T) {
	fs := pflag.NewFlagSet("redis", pflag.ContinueOnError)
	opts := NewRedisOptions()
	opts.AddFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse(nil) = %v, want nil", err)
	}
	if got, want := opts.Host, "127.0.0.1"; got != want {
		t.Errorf("RedisOptions.Host = %q, want %q", got, want)
	}
	if got, want := opts.Port, 6379; got != want {
		t.Errorf("RedisOptions.Port = %d, want %d", got, want)
	}
	if got, want := opts.Password, ""; got != want {
		t.Errorf("RedisOptions.Password = %q, want %q", got, want)
	}
	if got, want := opts.DB, 0; got != want {
		t.Errorf("RedisOptions.DB = %d, want %d", got, want)
	}
}
