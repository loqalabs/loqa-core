package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type TelemetryConfig struct {
	LogLevel       string `yaml:"log_level"`
	OTLPEndpoint   string `yaml:"otlp_endpoint"`
	OTLPInsecure   bool   `yaml:"otlp_insecure"`
	PrometheusBind string `yaml:"prometheus_bind"`
}

type HTTPConfig struct {
	Bind string `yaml:"bind"`
	Port int    `yaml:"port"`
}

type Config struct {
	RuntimeName string           `yaml:"runtime_name"`
	Environment string           `yaml:"environment"`
	HTTP        HTTPConfig       `yaml:"http"`
	Telemetry   TelemetryConfig  `yaml:"telemetry"`
	Bus         BusConfig        `yaml:"bus"`
	Node        NodeConfig       `yaml:"node"`
	EventStore  EventStoreConfig `yaml:"event_store"`
	STT         STTConfig        `yaml:"stt"`
	LLM         LLMConfig        `yaml:"llm"`
	TTS         TTSConfig        `yaml:"tts"`
}

type BusConfig struct {
	Servers        []string `yaml:"servers"`
	Username       string   `yaml:"username"`
	Password       string   `yaml:"password"`
	Token          string   `yaml:"token"`
	TLSInsecure    bool     `yaml:"tls_insecure"`
	ConnectTimeout int      `yaml:"connect_timeout_ms"`
}

type NodeConfig struct {
	ID                string           `yaml:"id"`
	Role              string           `yaml:"role"`
	HeartbeatInterval int              `yaml:"heartbeat_interval_ms"`
	HeartbeatTimeout  int              `yaml:"heartbeat_timeout_ms"`
	Capabilities      []NodeCapability `yaml:"capabilities"`
}

type NodeCapability struct {
	Name       string            `yaml:"name"`
	Tier       string            `yaml:"tier"`
	Attributes map[string]string `yaml:"attributes"`
}

type EventStoreConfig struct {
	Path          string `yaml:"path"`
	RetentionMode string `yaml:"retention_mode"`
	RetentionDays int    `yaml:"retention_days"`
	MaxSessions   int    `yaml:"max_sessions"`
	VacuumOnStart bool   `yaml:"vacuum_on_start"`
}

type STTConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Mode            string `yaml:"mode"`
	Command         string `yaml:"command"`
	ModelPath       string `yaml:"model_path"`
	Language        string `yaml:"language"`
	SampleRate      int    `yaml:"sample_rate"`
	Channels        int    `yaml:"channels"`
	FrameDurationMS int    `yaml:"frame_duration_ms"`
	PartialEveryMS  int    `yaml:"partial_every_ms"`
	PublishInterim  bool   `yaml:"publish_interim"`
}

type LLMConfig struct {
	Enabled       bool    `yaml:"enabled"`
	Mode          string  `yaml:"mode"` // mock, ollama, exec
	Endpoint      string  `yaml:"endpoint"`
	Command       string  `yaml:"command"`
	ModelFast     string  `yaml:"model_fast"`
	ModelBalanced string  `yaml:"model_balanced"`
	DefaultTier   string  `yaml:"default_tier"`
	MaxTokens     int     `yaml:"max_tokens"`
	Temperature   float64 `yaml:"temperature"`
}

type TTSConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Mode            string `yaml:"mode"`
	Command         string `yaml:"command"`
	Voice           string `yaml:"voice"`
	SampleRate      int    `yaml:"sample_rate"`
	Channels        int    `yaml:"channels"`
	ChunkDurationMS int    `yaml:"chunk_duration_ms"`
}

func Default() Config {
	return Config{
		RuntimeName: "loqa-runtime",
		Environment: "development",
		HTTP: HTTPConfig{
			Bind: "0.0.0.0",
			Port: 8080,
		},
		Telemetry: TelemetryConfig{
			LogLevel:       "info",
			OTLPEndpoint:   "",
			OTLPInsecure:   true,
			PrometheusBind: ":9091",
		},
		Bus: BusConfig{
			Servers:        []string{"nats://localhost:4222"},
			ConnectTimeout: 2000,
		},
		Node: NodeConfig{
			ID:                "loqa-node-1",
			Role:              "runtime",
			HeartbeatInterval: 2000,
			HeartbeatTimeout:  6000,
			Capabilities: []NodeCapability{
				{Name: "runtime.core", Tier: "balanced"},
			},
		},
		EventStore: EventStoreConfig{
			Path:          "./data/loqa-events.db",
			RetentionMode: "session",
			RetentionDays: 30,
			MaxSessions:   10000,
		},
		STT: STTConfig{
			Enabled:         false,
			Mode:            "mock",
			SampleRate:      16000,
			Channels:        1,
			FrameDurationMS: 20,
			PartialEveryMS:  800,
		},
		LLM: LLMConfig{
			Enabled:       false,
			Mode:          "mock",
			Endpoint:      "http://localhost:11434",
			ModelFast:     "llama3.2:latest",
			ModelBalanced: "llama3.2:latest",
			DefaultTier:   "balanced",
			MaxTokens:     256,
			Temperature:   0.7,
		},
		TTS: TTSConfig{
			Enabled:         false,
			Mode:            "mock",
			SampleRate:      22050,
			Channels:        1,
			ChunkDurationMS: 400,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	if path != "" {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return cfg, fmt.Errorf("config file not found: %w", err)
			}
			return cfg, fmt.Errorf("failed to read config file: %w", err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	applyEnvOverrides(&cfg)
	if err := validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	overrideString(&cfg.RuntimeName, "LOQA_RUNTIME_NAME")
	overrideString(&cfg.Environment, "LOQA_RUNTIME_ENVIRONMENT")
	overrideString(&cfg.HTTP.Bind, "LOQA_HTTP_BIND")
	overrideInt(&cfg.HTTP.Port, "LOQA_HTTP_PORT")
	overrideString(&cfg.Telemetry.LogLevel, "LOQA_TELEMETRY_LOG_LEVEL")
	overrideString(&cfg.Telemetry.OTLPEndpoint, "LOQA_TELEMETRY_OTLP_ENDPOINT")
	overrideBool(&cfg.Telemetry.OTLPInsecure, "LOQA_TELEMETRY_OTLP_INSECURE")
	overrideString(&cfg.Telemetry.PrometheusBind, "LOQA_TELEMETRY_PROMETHEUS_BIND")
	overrideStringSlice(&cfg.Bus.Servers, "LOQA_BUS_SERVERS")
	overrideString(&cfg.Bus.Username, "LOQA_BUS_USERNAME")
	overrideString(&cfg.Bus.Password, "LOQA_BUS_PASSWORD")
	overrideString(&cfg.Bus.Token, "LOQA_BUS_TOKEN")
	overrideBool(&cfg.Bus.TLSInsecure, "LOQA_BUS_TLS_INSECURE")
	overrideInt(&cfg.Bus.ConnectTimeout, "LOQA_BUS_CONNECT_TIMEOUT_MS")
	overrideString(&cfg.Node.ID, "LOQA_NODE_ID")
	overrideString(&cfg.Node.Role, "LOQA_NODE_ROLE")
	overrideInt(&cfg.Node.HeartbeatInterval, "LOQA_NODE_HEARTBEAT_INTERVAL_MS")
	overrideInt(&cfg.Node.HeartbeatTimeout, "LOQA_NODE_HEARTBEAT_TIMEOUT_MS")
	overrideString(&cfg.EventStore.Path, "LOQA_EVENT_STORE_PATH")
	overrideString(&cfg.EventStore.RetentionMode, "LOQA_EVENT_STORE_RETENTION_MODE")
	overrideInt(&cfg.EventStore.RetentionDays, "LOQA_EVENT_STORE_RETENTION_DAYS")
	overrideInt(&cfg.EventStore.MaxSessions, "LOQA_EVENT_STORE_MAX_SESSIONS")
	overrideBool(&cfg.EventStore.VacuumOnStart, "LOQA_EVENT_STORE_VACUUM_ON_START")
	overrideBool(&cfg.STT.Enabled, "LOQA_STT_ENABLED")
	overrideString(&cfg.STT.Mode, "LOQA_STT_MODE")
	overrideString(&cfg.STT.Command, "LOQA_STT_COMMAND")
	overrideString(&cfg.STT.ModelPath, "LOQA_STT_MODEL_PATH")
	overrideString(&cfg.STT.Language, "LOQA_STT_LANGUAGE")
	overrideInt(&cfg.STT.SampleRate, "LOQA_STT_SAMPLE_RATE")
	overrideInt(&cfg.STT.Channels, "LOQA_STT_CHANNELS")
	overrideInt(&cfg.STT.FrameDurationMS, "LOQA_STT_FRAME_DURATION_MS")
	overrideInt(&cfg.STT.PartialEveryMS, "LOQA_STT_PARTIAL_EVERY_MS")
	overrideBool(&cfg.STT.PublishInterim, "LOQA_STT_PUBLISH_INTERIM")
	overrideBool(&cfg.LLM.Enabled, "LOQA_LLM_ENABLED")
	overrideString(&cfg.LLM.Mode, "LOQA_LLM_MODE")
	overrideString(&cfg.LLM.Endpoint, "LOQA_LLM_ENDPOINT")
	overrideString(&cfg.LLM.Command, "LOQA_LLM_COMMAND")
	overrideString(&cfg.LLM.ModelFast, "LOQA_LLM_MODEL_FAST")
	overrideString(&cfg.LLM.ModelBalanced, "LOQA_LLM_MODEL_BALANCED")
	overrideString(&cfg.LLM.DefaultTier, "LOQA_LLM_DEFAULT_TIER")
	overrideInt(&cfg.LLM.MaxTokens, "LOQA_LLM_MAX_TOKENS")
	overrideFloat(&cfg.LLM.Temperature, "LOQA_LLM_TEMPERATURE")
	overrideBool(&cfg.TTS.Enabled, "LOQA_TTS_ENABLED")
	overrideString(&cfg.TTS.Mode, "LOQA_TTS_MODE")
	overrideString(&cfg.TTS.Command, "LOQA_TTS_COMMAND")
	overrideString(&cfg.TTS.Voice, "LOQA_TTS_VOICE")
	overrideInt(&cfg.TTS.SampleRate, "LOQA_TTS_SAMPLE_RATE")
	overrideInt(&cfg.TTS.Channels, "LOQA_TTS_CHANNELS")
	overrideInt(&cfg.TTS.ChunkDurationMS, "LOQA_TTS_CHUNK_DURATION_MS")
}

func overrideString(target *string, envKey string) {
	if value, ok := os.LookupEnv(envKey); ok && strings.TrimSpace(value) != "" {
		*target = value
	}
}

func overrideInt(target *int, envKey string) {
	if value, ok := os.LookupEnv(envKey); ok {
		if parsed, err := strconv.Atoi(value); err == nil {
			*target = parsed
		}
	}
}

func overrideBool(target *bool, envKey string) {
	if value, ok := os.LookupEnv(envKey); ok {
		if parsed, err := strconv.ParseBool(value); err == nil {
			*target = parsed
		}
	}
}

func overrideStringSlice(target *[]string, envKey string) {
	if value, ok := os.LookupEnv(envKey); ok {
		parts := strings.Split(value, ",")
		var trimmed []string
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				trimmed = append(trimmed, s)
			}
		}
		if len(trimmed) > 0 {
			*target = trimmed
		}
	}
}

func overrideFloat(target *float64, envKey string) {
	if value, ok := os.LookupEnv(envKey); ok {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			*target = parsed
		}
	}
}

func validate(cfg Config) error {
	if cfg.RuntimeName == "" {
		return errors.New("runtime_name must not be empty")
	}
	if cfg.HTTP.Port <= 0 || cfg.HTTP.Port > 65535 {
		return errors.New("http.port must be between 1 and 65535")
	}
	if len(cfg.Bus.Servers) == 0 {
		return errors.New("bus.servers must not be empty")
	}
	if cfg.Node.ID == "" {
		return errors.New("node.id must not be empty")
	}
	if cfg.Node.HeartbeatInterval <= 0 {
		return errors.New("node.heartbeat_interval_ms must be positive")
	}
	if cfg.Node.HeartbeatTimeout <= cfg.Node.HeartbeatInterval {
		return errors.New("node.heartbeat_timeout_ms must be greater than heartbeat interval")
	}
	if len(cfg.Node.Capabilities) == 0 {
		return errors.New("node.capabilities must not be empty")
	}
	if cfg.EventStore.Path == "" {
		return errors.New("event_store.path must not be empty")
	}
	switch cfg.EventStore.RetentionMode {
	case "ephemeral", "session", "persistent":
		// ok
	default:
		return errors.New("event_store.retention_mode must be one of ephemeral|session|persistent")
	}
	if cfg.EventStore.RetentionDays < 0 {
		return errors.New("event_store.retention_days must be >= 0")
	}
	if cfg.Telemetry.PrometheusBind == "" {
		return errors.New("telemetry.prometheus_bind must not be empty")
	}
	if cfg.STT.Enabled {
		if cfg.STT.SampleRate <= 0 {
			return errors.New("stt.sample_rate must be positive")
		}
		if cfg.STT.Channels <= 0 {
			return errors.New("stt.channels must be positive")
		}
		if cfg.STT.Mode == "exec" && cfg.STT.Command == "" {
			return errors.New("stt.command must be set when mode=exec")
		}
	}
	if cfg.LLM.Enabled {
		switch cfg.LLM.Mode {
		case "mock", "ollama", "exec":
		default:
			return errors.New("llm.mode must be one of mock|ollama|exec")
		}
		if cfg.LLM.Mode == "ollama" && cfg.LLM.Endpoint == "" {
			return errors.New("llm.endpoint must be set when mode=ollama")
		}
		if cfg.LLM.Mode == "exec" && cfg.LLM.Command == "" {
			return errors.New("llm.command must be set when mode=exec")
		}
		if cfg.LLM.MaxTokens < 0 {
			return errors.New("llm.max_tokens must be >= 0")
		}
	}
	if cfg.TTS.Enabled {
		switch cfg.TTS.Mode {
		case "mock", "exec":
		default:
			return errors.New("tts.mode must be one of mock|exec")
		}
		if cfg.TTS.Mode == "exec" && cfg.TTS.Command == "" {
			return errors.New("tts.command must be set when mode=exec")
		}
		if cfg.TTS.SampleRate <= 0 {
			return errors.New("tts.sample_rate must be positive")
		}
		if cfg.TTS.Channels <= 0 {
			return errors.New("tts.channels must be positive")
		}
	}
	return nil
}
