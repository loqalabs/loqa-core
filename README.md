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

The bootstrap process exposes `/healthz` and `/readyz` endpoints and initializes OpenTelemetry tracing with a local stdout exporter. See `cmd/loqad --help` for additional flags.

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
