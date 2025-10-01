# Loqa Core

The core runtime, control plane, SDKs, documentation, and protocol schemas for Loqa - a local-first, open-core ambient intelligence platform.

## Overview

Loqa Core contains the foundational components for building and running a distributed, privacy-first AI system on local hardware. This monorepo includes:

- **Runtime**: Core execution engine for orchestrating AI services
- **Control Plane**: Service discovery, load balancing, and cluster management
- **SDKs**: Client libraries for various languages
- **Documentation**: Technical docs and API references
- **Proto Schemas**: Protocol definitions for inter-service communication

## A Composable, Ethical Open Core

Loqa is an open-source ambient intelligence platform designed to run locally, adapt to your life, and respect your data.

- **Composable Open Core:** The runtime, protocols, and tooling stay MIT-licensed forever. You can self-host, fork, and embed without restrictions.
- **Modular extensibility:** Skills and adapters plug in like VS Code extensions. Start with the [authoring guide](skills/AUTHORING_GUIDE.md) and the [skills spec](https://github.com/loqalabs/loqa-core/blob/main/docs/skills/SPEC.md).
- **Loqa Studio add-ons:** Optional persona packs, premium skills, encrypted Loqa Cloud sync, and support subscriptions enrich the experience without gating your freedom.

We call this model **Composable Open Core**, backed by a commitment to **ethical monetization**. No bait-and-switch. No forced telemetry. No locked-in silos. Just intelligent software you can trust—and build on.

Want to contribute? Explore the [Extension Labs resources](https://github.com/loqalabs/loqa-meta/blob/main/community/extension-labs/README.md), read the [contributor guide](https://github.com/loqalabs/loqa-meta/blob/main/community/contributing-guide.md), or weigh in on the marketplace RFC ([RFC-0003](https://github.com/loqalabs/loqa-meta/blob/main/rfcs/RFC-0003_loqa_marketplace_mvp.md)).

## Downloads

- **Nightly snapshots:** Every day the [Nightly builds](https://github.com/loqalabs/loqa-core/actions/workflows/nightly.yml) workflow publishes artifacts that contain precompiled `loqad`, `loqa-skill`, sample configs, docs, and the TinyGo example skill packages. Download the archive that matches your platform (e.g., `loqa-core_nightly-YYYYMMDD_linux_amd64.tar.gz`), verify it with the accompanying `.sha256`, and extract it with `tar -xzf`.
- **Tagged releases:** Pushing a `v*` tag produces versioned bundles via the [Release workflow](https://github.com/loqalabs/loqa-core/actions/workflows/release.yml). These are attached automatically to the corresponding GitHub Release page.

## Getting Started

- **Read the Quickstart:** Follow the step-by-step guide in [`docs/GETTING_STARTED.md`](docs/GETTING_STARTED.md) to install prerequisites, build the sample skills, and publish your first events.
- **Launch the runtime quickly:**

  ```bash
  make skills   # builds TinyGo examples and validates manifests
  make run      # runs go run ./cmd/loqad --config ./config/example.yaml
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

## Documentation

- **Installation guide:** [`docs/INSTALLATION.md`](docs/INSTALLATION.md) covers prerequisites, configuration, and verifying your environment.
- **Quickstart:** [`docs/GETTING_STARTED.md`](docs/GETTING_STARTED.md) walks through the timer and smart-home skills, plus the optional voice pipeline loop.
- **Architecture overview:** [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) explains the runtime, message bus subjects, and extension points.
- **Skills spec:** [`docs/skills/SPEC.md`](docs/skills/SPEC.md) defines the manifest schema and host ABI for `host_version: v1`.
- **Hosted experience:** The same content is available on [loqa.ambiware.ai/docs](https://loqa.ambiware.ai/docs) with a responsive layout and copy-ready commands.

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

## Skills

Skill packages declare metadata, runtime, and permissions in a `skill.yaml` manifest. Validate locally with:

```bash
go run ./cmd/loqa-skill validate --file skills/examples/timer/skill.yaml
```

When `skills.enabled` is true in `config/example.yaml`, the runtime loads manifests from `skills.directory`, subscribes to declared NATS subjects, and invokes the corresponding WASM module for each event. Skills publish responses via the host API—`host.Publish` enforces both the `bus:publish` permission and the subjects enumerated in `capabilities.bus.publish`. All invocations and publish operations are recorded in the event-store audit log under the `skill:*` sessions configured by `skills.audit_privacy_scope`.

See [`skills/AUTHORING_GUIDE.md`](skills/AUTHORING_GUIDE.md) for a step-by-step walkthrough on building TinyGo skills, defining manifests, and testing locally.

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
