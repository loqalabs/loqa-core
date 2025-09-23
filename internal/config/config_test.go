package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Bus.Servers[0] != "nats://localhost:4222" {
		t.Fatalf("expected default server, got %v", cfg.Bus.Servers)
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("LOQA_BUS_SERVERS", "nats://one:4222, nats://two:4222")
	t.Setenv("LOQA_BUS_USERNAME", "alice")
	t.Setenv("LOQA_BUS_PASSWORD", "secret")
	t.Setenv("LOQA_BUS_TLS_INSECURE", "true")
	t.Setenv("LOQA_BUS_CONNECT_TIMEOUT_MS", "5000")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Bus.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %v", cfg.Bus.Servers)
	}
	if cfg.Bus.Username != "alice" || cfg.Bus.Password != "secret" {
		t.Fatalf("expected credentials override")
	}
	if !cfg.Bus.TLSInsecure {
		t.Fatal("expected tls insecure override true")
	}
	if cfg.Bus.ConnectTimeout != 5000 {
		t.Fatalf("expected timeout 5000, got %d", cfg.Bus.ConnectTimeout)
	}

}
