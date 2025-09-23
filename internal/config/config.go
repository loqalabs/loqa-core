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
	LogLevel     string `yaml:"log_level"`
	OTLPEndpoint string `yaml:"otlp_endpoint"`
}

type HTTPConfig struct {
	Bind string `yaml:"bind"`
	Port int    `yaml:"port"`
}

type Config struct {
	RuntimeName string          `yaml:"runtime_name"`
	Environment string          `yaml:"environment"`
	HTTP        HTTPConfig      `yaml:"http"`
	Telemetry   TelemetryConfig `yaml:"telemetry"`
	Bus         BusConfig       `yaml:"bus"`
	Node        NodeConfig      `yaml:"node"`
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
	ID                 string             `yaml:"id"`
	Role               string             `yaml:"role"`
	HeartbeatInterval  int                `yaml:"heartbeat_interval_ms"`
	HeartbeatTimeout   int                `yaml:"heartbeat_timeout_ms"`
	Capabilities       []NodeCapability   `yaml:"capabilities"`
}

type NodeCapability struct {
	Name       string            `yaml:"name"`
	Tier       string            `yaml:"tier"`
	Attributes map[string]string `yaml:"attributes"`
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
			LogLevel:     "info",
			OTLPEndpoint: "",
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
	return nil
}
