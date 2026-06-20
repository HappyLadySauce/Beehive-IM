package rabbitmq

import "testing"

func TestNormalizeUsesEnvURL(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "amqp://user:pass@127.0.0.1:5672/")
	cfg := Config{DisableDefaultLocalURL: true}.Normalize()
	if cfg.URL != "amqp://user:pass@127.0.0.1:5672/" {
		t.Fatalf("URL = %q, want env URL", cfg.URL)
	}
}

func TestNormalizeAppliesDefaults(t *testing.T) {
	cfg := Config{}.Normalize()
	if cfg.URL != defaultURL {
		t.Fatalf("URL = %q, want %q", cfg.URL, defaultURL)
	}
	if cfg.Exchange != defaultExchange {
		t.Fatalf("Exchange = %q, want %q", cfg.Exchange, defaultExchange)
	}
}
