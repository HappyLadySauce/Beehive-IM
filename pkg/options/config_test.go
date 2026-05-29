package options

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// TestInitRegistersConfigFlagOnPflagCommandLine ensures the package init registers --config on the global pflag set.
// TestInitRegistersConfigFlagOnPflagCommandLine 确认 init 在全局 pflag 上注册了 --config。
func TestInitRegistersConfigFlagOnPflagCommandLine(t *testing.T) {
	f := pflag.Lookup("config")
	if f == nil {
		t.Errorf("pflag.Lookup(config) = nil, want non-nil *pflag.Flag")
		return
	}
	if got, want := f.Shorthand, "f"; got != want {
		t.Errorf("config flag Shorthand = %q, want %q", got, want)
	}
	if !strings.Contains(f.Usage, "Read configuration from specified") {
		t.Errorf("config flag Usage = %q, want substring %q", f.Usage, "Read configuration from specified")
	}
}

// TestAddConfigFlagAddsFlagToFlagSet wires the shared --config flag into a custom FlagSet by reference.
// TestAddConfigFlagAddsFlagToFlagSet 验证共享 --config 以引用方式挂入自定义 FlagSet。
func TestAddConfigFlagAddsFlagToFlagSet(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
AddConfigFlag(fs, "beehive-blog")
	got := fs.Lookup("config")
	if got == nil {
		t.Errorf("fs.Lookup(config) = nil, want non-nil *pflag.Flag")
		return
	}
	global := pflag.Lookup("config")
	if global == nil {
		t.Errorf("pflag.Lookup(config) = nil, want non-nil *pflag.Flag")
		return
	}
	if got != global {
		t.Errorf("fs.Lookup(config) pointer = %p, want same as global %p", got, global)
	}
}

// TestAddConfigFlagBindsEnvWithBasenamePrefix checks env prefix and key replacer for hyphenated basenames.
// TestAddConfigFlagBindsEnvWithBasenamePrefix 校验带连字符 basename 的环境变量前缀与键替换。
func TestAddConfigFlagBindsEnvWithBasenamePrefix(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
AddConfigFlag(fs, "beehive-blog")
	t.Setenv("BEEHIVE_BLOG_FOO_BAR", "from-env")

	if got, want := viper.GetString("foo.bar"), "from-env"; got != want {
		t.Errorf("viper.GetString(foo.bar) = %q, want %q", got, want)
	}
}

// TestAddConfigFlagSingleSegmentBasename ensures a single-segment basename does not panic and still binds env.
// TestAddConfigFlagSingleSegmentBasename 确认单段 basename 不 panic 且仍能绑定环境变量。
func TestAddConfigFlagSingleSegmentBasename(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
AddConfigFlag(fs, "app")
	t.Setenv("APP_X", "y")

	if got, want := viper.GetString("x"), "y"; got != want {
		t.Errorf("viper.GetString(x) = %q, want %q", got, want)
	}
}
