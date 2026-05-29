package options

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestInsecureServingValidateJoinsAllMissingFields(t *testing.T) {
	opts := &InsecureServingOptions{}
	err := opts.Validate()
	if err == nil {
		t.Errorf("Validate() = nil, want non-nil validation error")
		return
	}
	msg := err.Error()
	if !strings.Contains(msg, "bind-address is required") {
		t.Errorf("Validate() error = %q, want substring %q", msg, "bind-address is required")
	}
	if !strings.Contains(msg, "bind-port is required") {
		t.Errorf("Validate() error = %q, want substring %q", msg, "bind-port is required")
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Errorf("Validate() error type = %T, want joined error with Unwrap() []error", err)
		return
	}
	if got, want := len(joined.Unwrap()), 2; got != want {
		t.Errorf("len(Unwrap()) = %d, want %d", got, want)
	}
}

func TestInsecureServingValidateReturnsSingleErrorWhenOneFieldMissing(t *testing.T) {
	opts := &InsecureServingOptions{BindAddress: "127.0.0.1"}
	err := opts.Validate()
	if err == nil {
		t.Errorf("Validate() = nil, want non-nil validation error")
		return
	}
	if got, want := err.Error(), "bind-port is required"; got != want {
		t.Errorf("Validate() error = %q, want %q", got, want)
	}
}

// TestInsecureServingAddFlagsDefaults verifies default flag values and shorthand letters after Parse(nil).
// TestInsecureServingAddFlagsDefaults 校验 Parse(nil) 后的默认值与短标志字母。
func TestInsecureServingAddFlagsDefaults(t *testing.T) {
	fs := pflag.NewFlagSet("insecure", pflag.ContinueOnError)
	opts := NewInsecureServingOptions()
	opts.AddFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse(nil) = %v, want nil", err)
	}
	if got, want := opts.BindAddress, "127.0.0.1"; got != want {
		t.Errorf("InsecureServingOptions.BindAddress = %q, want %q", got, want)
	}
	if got, want := opts.BindPort, 8080; got != want {
		t.Errorf("InsecureServingOptions.BindPort = %d, want %d", got, want)
	}
	if f := fs.Lookup("bind-address"); f == nil {
		t.Errorf("fs.Lookup(bind-address) = nil, want non-nil *pflag.Flag")
	} else if got, want := f.Shorthand, "b"; got != want {
		t.Errorf("bind-address Shorthand = %q, want %q", got, want)
	}
	if f := fs.Lookup("bind-port"); f == nil {
		t.Errorf("fs.Lookup(bind-port) = nil, want non-nil *pflag.Flag")
	} else if got, want := f.Shorthand, "p"; got != want {
		t.Errorf("bind-port Shorthand = %q, want %q", got, want)
	}
}
