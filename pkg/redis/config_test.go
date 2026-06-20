package redis

import "testing"

func TestNormalizeUsesEnvAddr(t *testing.T) {
	t.Setenv("REDIS_ADDR", "10.0.0.1:6379")
	cfg := Config{DisableDefaultLocalURL: true}.Normalize()
	if cfg.Addr != "10.0.0.1:6379" {
		t.Fatalf("Addr = %q, want env addr", cfg.Addr)
	}
}

func TestOptionsRejectsMissingAddr(t *testing.T) {
	_, err := Options(Config{DisableDefaultLocalURL: true})
	if err == nil {
		t.Fatal("Options() error = nil, want error")
	}
}

func TestOptionsAppliesDefaults(t *testing.T) {
	opts, err := Options(Config{})
	if err != nil {
		t.Fatalf("Options() error = %v", err)
	}
	if opts.Addr != defaultAddr {
		t.Fatalf("Addr = %q, want %q", opts.Addr, defaultAddr)
	}
}
