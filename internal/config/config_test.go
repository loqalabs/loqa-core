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
	t.Setenv("LOQA_NODE_ID", "test-node")
	t.Setenv("LOQA_NODE_ROLE", "runtime")
	t.Setenv("LOQA_NODE_HEARTBEAT_INTERVAL_MS", "1500")
	t.Setenv("LOQA_NODE_HEARTBEAT_TIMEOUT_MS", "5000")
	t.Setenv("LOQA_EVENT_STORE_PATH", "./tmp.db")
	t.Setenv("LOQA_EVENT_STORE_RETENTION_MODE", "persistent")
	t.Setenv("LOQA_EVENT_STORE_RETENTION_DAYS", "7")
	t.Setenv("LOQA_EVENT_STORE_MAX_SESSIONS", "123")
	t.Setenv("LOQA_EVENT_STORE_VACUUM_ON_START", "true")

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
	if cfg.Node.ID != "test-node" {
		t.Fatalf("expected node id override")
	}
	if cfg.Node.HeartbeatInterval != 1500 {
		t.Fatalf("expected heartbeat interval override")
	}
	if cfg.Node.HeartbeatTimeout != 5000 {
		t.Fatalf("expected heartbeat timeout override")
	}
	if cfg.EventStore.Path != "./tmp.db" {
		t.Fatalf("expected event store path override")
	}
	if cfg.EventStore.RetentionMode != "persistent" {
		t.Fatalf("expected event store retention mode override")
	}
	if cfg.EventStore.RetentionDays != 7 {
		t.Fatalf("expected event store retention days override")
	}
	if cfg.EventStore.MaxSessions != 123 {
		t.Fatalf("expected event store max sessions override")
	}
	if !cfg.EventStore.VacuumOnStart {
		t.Fatalf("expected event store vacuum flag override")
	}
}
