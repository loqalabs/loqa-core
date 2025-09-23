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

func validate(cfg Config) error {
	if cfg.RuntimeName == "" {
		return errors.New("runtime_name must not be empty")
	}
	if cfg.HTTP.Port <= 0 || cfg.HTTP.Port > 65535 {
		return errors.New("http.port must be between 1 and 65535")
	}
	return nil
}
