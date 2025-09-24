# Loqa Core

The core runtime, control plane, SDKs, documentation, and protocol schemas for Loqa - a local-first, open-core ambient intelligence platform.

## Overview

Loqa Core contains the foundational components for building and running a distributed, privacy-first AI system on local hardware. This monorepo includes:

- **Runtime**: Core execution engine for orchestrating AI services
- **Control Plane**: Service discovery, load balancing, and cluster management
- **SDKs**: Client libraries for various languages
- **Documentation**: Technical docs and API references
- **Proto Schemas**: Protocol definitions for inter-service communication

## Getting Started

Install prerequisites (Go 1.21+). Then:

```bash
go run ./cmd/loqad --config ./config/example.yaml
```

If no config file is supplied the runtime loads defaults and respects the following environment overrides:

- `LOQA_RUNTIME_NAME`
- `LOQA_RUNTIME_ENVIRONMENT`
- `LOQA_HTTP_BIND`
- `LOQA_HTTP_PORT`
- `LOQA_TELEMETRY_LOG_LEVEL`
- `LOQA_TELEMETRY_OTLP_ENDPOINT`
- `LOQA_TELEMETRY_OTLP_INSECURE`
- `LOQA_TELEMETRY_PROMETHEUS_BIND`
- `LOQA_BUS_SERVERS` (comma-separated list)
- `LOQA_BUS_USERNAME`
- `LOQA_BUS_PASSWORD`
- `LOQA_BUS_TOKEN`
- `LOQA_BUS_TLS_INSECURE`
- `LOQA_BUS_CONNECT_TIMEOUT_MS`
- `LOQA_NODE_ID`
- `LOQA_NODE_ROLE`
- `LOQA_NODE_HEARTBEAT_INTERVAL_MS`
- `LOQA_NODE_HEARTBEAT_TIMEOUT_MS`
- `LOQA_EVENT_STORE_PATH`
- `LOQA_EVENT_STORE_RETENTION_MODE`
- `LOQA_EVENT_STORE_RETENTION_DAYS`
- `LOQA_EVENT_STORE_MAX_SESSIONS`
- `LOQA_EVENT_STORE_VACUUM_ON_START`

The bootstrap process exposes `/healthz` and `/readyz` endpoints and initializes OpenTelemetry tracing with a local stdout exporter. See `cmd/loqad --help` for additional flags.

> **Note:** Loqa Core expects a NATS server with JetStream enabled to be running at the configured `bus.servers` endpoint (default `nats://localhost:4222`). You can start one locally with `nats-server --js` or the official Docker image.

An on-disk SQLite event store is created at `event_store.path` (default `./data/loqa-events.db`) unless `event_store.retention_mode` is set to `ephemeral`. Use `session` for local replay debugging or `persistent` to honor retention windows (days and max sessions).

To visualize traces/metrics/logs locally, set `LOQA_TELEMETRY_OTLP_ENDPOINT=localhost:4317` and run the docker-compose stack under `observability/`.

## Architecture

Loqa is designed as a modular, distributed system that can scale across multiple local nodes:

- Speech-to-Text (STT) services
- Large Language Model (LLM) inference
- Text-to-Speech (TTS) services
- Skill execution and plugin management
- Message bus for inter-service communication

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to contribute to this project.

## License

This repository is released under the [MIT License](LICENSE). Commercial extensions will live in separate repositories without impacting the open-source core.

## About Ambiware Labs

Loqa is developed by [Ambiware Labs](https://ambiware.ai), building the future of local-first ambient intelligence.
