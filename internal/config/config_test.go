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
	if cfg.Telemetry.PrometheusBind != ":9091" {
		t.Fatalf("unexpected default prometheus bind: %s", cfg.Telemetry.PrometheusBind)
	}
	if cfg.STT.Mode != "mock" {
		t.Fatalf("expected default STT mode mock, got %s", cfg.STT.Mode)
	}
	if cfg.LLM.Mode != "mock" {
		t.Fatalf("expected default LLM mode mock, got %s", cfg.LLM.Mode)
	}
	if cfg.TTS.Mode != "mock" {
		t.Fatalf("expected default TTS mode mock, got %s", cfg.TTS.Mode)
	}
	if !cfg.Router.Enabled {
		t.Fatalf("expected router enabled by default")
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
	t.Setenv("LOQA_STT_ENABLED", "true")
	t.Setenv("LOQA_STT_MODE", "exec")
	t.Setenv("LOQA_STT_COMMAND", "python3 scripts/stt/transcribe.py")
	t.Setenv("LOQA_STT_MODEL_PATH", "./models/ggml-base.bin")
	t.Setenv("LOQA_STT_LANGUAGE", "en")
	t.Setenv("LOQA_STT_SAMPLE_RATE", "44100")
	t.Setenv("LOQA_STT_CHANNELS", "2")
	t.Setenv("LOQA_STT_FRAME_DURATION_MS", "30")
	t.Setenv("LOQA_STT_PARTIAL_EVERY_MS", "500")
	t.Setenv("LOQA_STT_PUBLISH_INTERIM", "true")
	t.Setenv("LOQA_LLM_ENABLED", "true")
	t.Setenv("LOQA_LLM_MODE", "ollama")
	t.Setenv("LOQA_LLM_ENDPOINT", "http://localhost:11434")
	t.Setenv("LOQA_LLM_MODEL_FAST", "llama3.1:8b")
	t.Setenv("LOQA_LLM_MODEL_BALANCED", "llama3.1:70b")
	t.Setenv("LOQA_LLM_DEFAULT_TIER", "fast")
	t.Setenv("LOQA_LLM_MAX_TOKENS", "128")
	t.Setenv("LOQA_LLM_TEMPERATURE", "0.5")
	t.Setenv("LOQA_TTS_ENABLED", "true")
	t.Setenv("LOQA_TTS_MODE", "exec")
	t.Setenv("LOQA_TTS_COMMAND", "python3 tts/kokoro.py")
	t.Setenv("LOQA_TTS_VOICE", "en-US")
	t.Setenv("LOQA_TTS_SAMPLE_RATE", "48000")
	t.Setenv("LOQA_TTS_CHANNELS", "2")
	t.Setenv("LOQA_TTS_CHUNK_DURATION_MS", "200")
	t.Setenv("LOQA_ROUTER_ENABLED", "true")
	t.Setenv("LOQA_ROUTER_DEFAULT_TIER", "fast")
	t.Setenv("LOQA_ROUTER_DEFAULT_VOICE", "en-GB")
	t.Setenv("LOQA_ROUTER_TARGET", "livingroom")

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
	if !cfg.STT.Enabled || cfg.STT.Mode != "exec" || cfg.STT.Command == "" {
		t.Fatalf("expected STT overrides applied")
	}
	if cfg.STT.SampleRate != 44100 || cfg.STT.Channels != 2 {
		t.Fatalf("expected STT sample overrides, got %d/%d", cfg.STT.SampleRate, cfg.STT.Channels)
	}
	if cfg.STT.PartialEveryMS != 500 || !cfg.STT.PublishInterim {
		t.Fatalf("expected STT partial overrides")
	}
	if !cfg.LLM.Enabled || cfg.LLM.Mode != "ollama" {
		t.Fatalf("expected LLM overrides")
	}
	if cfg.LLM.ModelFast != "llama3.1:8b" || cfg.LLM.MaxTokens != 128 {
		t.Fatalf("expected LLM model/limits override")
	}
	if cfg.LLM.Temperature != 0.5 {
		t.Fatalf("expected LLM temperature override, got %f", cfg.LLM.Temperature)
	}
	if !cfg.TTS.Enabled || cfg.TTS.Mode != "exec" {
		t.Fatalf("expected TTS overrides")
	}
	if cfg.TTS.SampleRate != 48000 || cfg.TTS.Channels != 2 {
		t.Fatalf("expected TTS sample overrides")
	}
	if cfg.TTS.ChunkDurationMS != 200 {
		t.Fatalf("expected TTS chunk override")
	}
	if !cfg.Router.Enabled || cfg.Router.DefaultTier != "fast" || cfg.Router.DefaultVoice != "en-GB" || cfg.Router.Target != "livingroom" {
		t.Fatalf("expected router overrides")
	}
}
