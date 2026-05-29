package options

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// TestPostgreOptionsValidate exercises PostgreOptions.Validate across required fields, port bounds, sslmode, and pool constraints.
// TestPostgreOptionsValidate 覆盖必填项、端口范围、sslmode 与连接池约束等 Validate 分支。
func TestPostgreOptionsValidate(t *testing.T) {
	tests := []struct {
		name       string
		opts       PostgreOptions
		wantNil    bool
		wantSubstr []string
		wantNoSub  []string
	}{
		{
			name: "valid_minimal",
			opts: PostgreOptions{
				Host: "127.0.0.1", Port: 5432, User: "u", DB: "d", SSLMode: "disable",
			},
			wantNil: true,
		},
		{
			name: "valid_with_pool",
			opts: PostgreOptions{
				Host: "h", Port: 1, User: "u", DB: "d", SSLMode: "require",
				MaxIdleConns: 5, MaxOpenConns: 10,
				ConnMaxLifetime: time.Hour, ConnMaxIdleTime: 30 * time.Minute,
			},
			wantNil: true,
		},
		{
			name:    "missing_all",
			opts:    PostgreOptions{},
			wantNil: false,
			wantSubstr: []string{
				"host is required",
				"port is required",
				"user is required",
				"db is required",
				"ssl-mode is required",
			},
		},
		{
			name:       "port_zero",
			opts:       PostgreOptions{Host: "h", Port: 0, User: "u", DB: "d", SSLMode: "disable"},
			wantNil:    false,
			wantSubstr: []string{"port is required"},
			wantNoSub:  []string{"port must be between"},
		},
		{
			name:       "port_below_min",
			opts:       PostgreOptions{Host: "h", Port: -1, User: "u", DB: "d", SSLMode: "disable"},
			wantNil:    false,
			wantSubstr: []string{"port must be between 1 and 65535 inclusive, got -1"},
		},
		{
			name:       "port_above_max",
			opts:       PostgreOptions{Host: "h", Port: 65536, User: "u", DB: "d", SSLMode: "disable"},
			wantNil:    false,
			wantSubstr: []string{"port must be between 1 and 65535 inclusive, got 65536"},
		},
		{
			name:       "unknown_ssl_mode",
			opts:       PostgreOptions{Host: "h", Port: 5432, User: "u", DB: "d", SSLMode: "bogus"},
			wantNil:    false,
			wantSubstr: []string{`ssl-mode must be one of disable, allow, prefer, require, verify-ca, verify-full, got "bogus"`},
		},
		{
			name:       "negative_idle_conns",
			opts:       PostgreOptions{Host: "h", Port: 5432, User: "u", DB: "d", SSLMode: "disable", MaxIdleConns: -1},
			wantNil:    false,
			wantSubstr: []string{"max-idle-conns must be >= 0, got -1"},
		},
		{
			name:       "negative_open_conns",
			opts:       PostgreOptions{Host: "h", Port: 5432, User: "u", DB: "d", SSLMode: "disable", MaxOpenConns: -1},
			wantNil:    false,
			wantSubstr: []string{"max-open-conns must be >= 0, got -1"},
		},
		{
			name:       "idle_gt_open",
			opts:       PostgreOptions{Host: "h", Port: 5432, User: "u", DB: "d", SSLMode: "disable", MaxIdleConns: 20, MaxOpenConns: 10},
			wantNil:    false,
			wantSubstr: []string{"max-idle-conns must be <= max-open-conns when max-open-conns > 0, got 20 and 10"},
		},
		{
			name:    "open_zero_allows_any_idle",
			opts:    PostgreOptions{Host: "h", Port: 5432, User: "u", DB: "d", SSLMode: "disable", MaxOpenConns: 0, MaxIdleConns: 50},
			wantNil: true,
		},
		{
			name:       "negative_lifetime",
			opts:       PostgreOptions{Host: "h", Port: 5432, User: "u", DB: "d", SSLMode: "disable", ConnMaxLifetime: -1 * time.Second},
			wantNil:    false,
			wantSubstr: []string{"conn-max-lifetime must be >= 0"},
		},
		{
			name:       "negative_idletime",
			opts:       PostgreOptions{Host: "h", Port: 5432, User: "u", DB: "d", SSLMode: "disable", ConnMaxIdleTime: -1 * time.Second},
			wantNil:    false,
			wantSubstr: []string{"conn-max-idle-time must be >= 0"},
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
			for _, s := range tt.wantNoSub {
				if strings.Contains(msg, s) {
					t.Errorf("Validate() error = %q, want no substring %q", msg, s)
				}
			}
		})
	}
}

// TestPostgreOptionsAddFlagsDefaults verifies default flag values after Parse(nil).
// TestPostgreOptionsAddFlagsDefaults 校验 Parse(nil) 后的默认标志值。
func TestPostgreOptionsAddFlagsDefaults(t *testing.T) {
	fs := pflag.NewFlagSet("postgre", pflag.ContinueOnError)
	opts := NewPostgreOptions()
	opts.AddFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse(nil) = %v, want nil", err)
	}
	if got, want := opts.Host, "127.0.0.1"; got != want {
		t.Errorf("PostgreOptions.Host = %q, want %q", got, want)
	}
	if got, want := opts.Port, 5432; got != want {
		t.Errorf("PostgreOptions.Port = %d, want %d", got, want)
	}
	if got, want := opts.User, "Beehive-Blog"; got != want {
		t.Errorf("PostgreOptions.User = %q, want %q", got, want)
	}
	if got, want := opts.DB, "Beehive-Blog"; got != want {
		t.Errorf("PostgreOptions.DB = %q, want %q", got, want)
	}
	if got, want := opts.SSLMode, "disable"; got != want {
		t.Errorf("PostgreOptions.SSLMode = %q, want %q", got, want)
	}
	if got, want := opts.MaxIdleConns, 10; got != want {
		t.Errorf("PostgreOptions.MaxIdleConns = %d, want %d", got, want)
	}
	if got, want := opts.MaxOpenConns, 100; got != want {
		t.Errorf("PostgreOptions.MaxOpenConns = %d, want %d", got, want)
	}
	if got, want := opts.ConnMaxLifetime, time.Hour; got != want {
		t.Errorf("PostgreOptions.ConnMaxLifetime = %v, want %v", got, want)
	}
	wantIdle := 30 * time.Minute
	if got := opts.ConnMaxIdleTime; got != wantIdle {
		t.Errorf("PostgreOptions.ConnMaxIdleTime = %v, want %v", got, wantIdle)
	}
}

// TestPostgreOptionsValidatePortBoundaries accepts min and max TCP ports.
// TestPostgreOptionsValidatePortBoundaries 校验端口上下界合法。
func TestPostgreOptionsValidatePortBoundaries(t *testing.T) {
	for _, port := range []int{1, 65535} {
		p := PostgreOptions{Host: "h", Port: port, User: "u", DB: "d", SSLMode: "disable"}
		if err := p.Validate(); err != nil {
			t.Errorf("Validate(port=%d) = %v, want nil", port, err)
		}
	}
}

// TestPostgreOptionsValidateJoinedErrors ensures all required-field messages appear on an empty struct.
// TestPostgreOptionsValidateJoinedErrors 确保空结构体校验结果包含全部必填项提示。
func TestPostgreOptionsValidateJoinedErrors(t *testing.T) {
	var p PostgreOptions
	err := p.Validate()
	if err == nil {
		t.Errorf("Validate() = nil, want non-nil joined error")
		return
	}
	msg := err.Error()
	for _, want := range []string{
		"host is required",
		"port is required",
		"user is required",
		"db is required",
		"ssl-mode is required",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("Validate() error = %q, want substring %q", msg, want)
		}
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Errorf("Validate() error type = %T, want joined error with Unwrap() []error", err)
		return
	}
	// Go may flatten nested joins; only require a non-empty Unwrap slice for joined errors.
	// Go 可能对嵌套 Join 做扁平化；此处仅要求 joined error 的 Unwrap 非空。
	if n := len(joined.Unwrap()); n < 1 {
		t.Errorf("len(Unwrap()) = %d, want >= 1", n)
	}
}
