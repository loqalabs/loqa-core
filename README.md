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
- `LOQA_STT_ENABLED`
- `LOQA_STT_MODE`
- `LOQA_STT_COMMAND`
- `LOQA_STT_MODEL_PATH`
- `LOQA_STT_LANGUAGE`
- `LOQA_STT_SAMPLE_RATE`
- `LOQA_STT_CHANNELS`
- `LOQA_STT_FRAME_DURATION_MS`
- `LOQA_STT_PARTIAL_EVERY_MS`
- `LOQA_STT_PUBLISH_INTERIM`
- `LOQA_LLM_ENABLED`
- `LOQA_LLM_MODE`
- `LOQA_LLM_ENDPOINT`
- `LOQA_LLM_COMMAND`
- `LOQA_LLM_MODEL_FAST`
- `LOQA_LLM_MODEL_BALANCED`
- `LOQA_LLM_DEFAULT_TIER`
- `LOQA_LLM_MAX_TOKENS`
- `LOQA_LLM_TEMPERATURE`
- `LOQA_TTS_ENABLED`
- `LOQA_TTS_MODE`
- `LOQA_TTS_COMMAND`
- `LOQA_TTS_VOICE`
- `LOQA_TTS_SAMPLE_RATE`
- `LOQA_TTS_CHANNELS`
- `LOQA_TTS_CHUNK_DURATION_MS`
- `LOQA_ROUTER_ENABLED`
- `LOQA_ROUTER_DEFAULT_TIER`
- `LOQA_ROUTER_DEFAULT_VOICE`
- `LOQA_ROUTER_TARGET`

The bootstrap process exposes `/healthz` and `/readyz` endpoints and initializes OpenTelemetry tracing with a local stdout exporter. See `cmd/loqad --help` for additional flags.

> **Note:** Loqa Core expects a NATS server with JetStream enabled to be running at the configured `bus.servers` endpoint (default `nats://localhost:4222`). You can start one locally with `nats-server --js` or the official Docker image.

An on-disk SQLite event store is created at `event_store.path` (default `./data/loqa-events.db`) unless `event_store.retention_mode` is set to `ephemeral`. Use `session` for local replay debugging or `persistent` to honor retention windows (days and max sessions).

To visualize traces/metrics/logs locally, set `LOQA_TELEMETRY_OTLP_ENDPOINT=localhost:4317` and run the docker-compose stack under `observability/`.

## Speech-to-Text (STT)

Set `stt.enabled: true` in the configuration to activate the streaming STT worker. Two modes are supported:

- `mock` — emits synthetic transcripts, useful for development without a model.
- `exec` — shells out to a command (for example the bundled `stt/faster_whisper.py` wrapper) that must return JSON `{ "text": "...", "confidence": 0.0 }` on stdout.

Example `stt` configuration:

```yaml
stt:
  enabled: true
  mode: exec
  command: "python3 stt/faster_whisper.py"
  model_path: ./models/ggml-base.bin
  language: en
  publish_interim: false
```

> Install dependencies with `pip install faster-whisper` and download an appropriate Whisper model. The helper script caches models between invocations.

## LLM Harness

Enable the language model service via `llm.enabled: true`. Two backends are available:

- `mock` – returns placeholder completions.
- `ollama` – streams completions from a local Ollama server (default endpoint `http://localhost:11434`).
- `exec` – shells out to a command that reads JSON from stdin and returns `{"content": "..."}` on stdout.

Example Ollama configuration:

```yaml
llm:
  enabled: true
  mode: ollama
  endpoint: http://localhost:11434
  model_fast: llama3.2:latest
  model_balanced: llama3.2:latest
  default_tier: balanced
  max_tokens: 256
  temperature: 0.7
```

The service subscribes to `nlu.request` messages and publishes streaming completions on `nlu.response.partial`/`nlu.response.final`.

## Text-to-Speech (TTS)

Enable with `tts.enabled: true`. Modes:

- `mock` – emits silent audio chunks for development.
- `exec` – shells out to a command (for example `python3 tts/kokoro_stub.py`) that reads JSON from stdin (`{"text": "hello", "voice": "en-US", "sample_rate": 22050, "channels": 1}`) and writes newline-delimited JSON responses containing base64-encoded PCM buffers (`{"pcm_base64": "...", "final": true}`).

```yaml
tts:
  enabled: true
  mode: exec
  command: "python3 tts/kokoro_stub.py"
  voice: en-US
  sample_rate: 22050
  channels: 1
  chunk_duration_ms: 400
```

Synthesized audio is published on `tts.audio`, with a `tts.done` signal once playback is complete at the originating device.

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
