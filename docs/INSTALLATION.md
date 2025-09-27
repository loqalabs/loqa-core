# Loqa Installation Guide

This guide covers the prerequisites and setup steps for running the Loqa runtime on a local workstation or lab environment. The instructions assume a single-node deployment for development; production clusters follow the same fundamentals but require hardened networking and monitoring.

## Supported platforms

> Nightly builds currently ship precompiled archives for Linux (amd64/arm64) and macOS (Intel/Apple Silicon). Windows support is planned but not yet automated.

- Linux (Ubuntu 22.04+, Fedora 39+, Debian 12)
- macOS 13+ (Intel or Apple Silicon)
- Docker or Podman is optional for running dependencies such as NATS or Ollama

## Prerequisites

| Requirement | Purpose |
| --- | --- |
| Go 1.22 or newer | Builds the `loqad` runtime and supporting CLI tooling. |
| TinyGo 0.39 or newer | Compiles the bundled WebAssembly sample skills. |
| Docker 24+ (optional) | Simplifies running NATS, Ollama, and observability dependencies. |
| Python 3.10+ (optional) | Executes the reference Whisper and Kokoro wrapper scripts. |
| NATS 2.10+ with JetStream | Core message bus for Loqa services. |

Install Go and TinyGo via your package manager or from their official installers. Example using Homebrew on macOS or Linux:

```bash
brew install go tinygo
```

Install the NATS CLI (optional but recommended) for quick testing:

```bash
brew install nats-io/nats/nats
```

## Step 1 – Obtain the runtime binaries

### Option A: Download a nightly archive

1. Visit the [Nightly builds workflow](https://github.com/ambiware-labs/loqa-core/actions/workflows/nightly.yml) and open the latest successful run.
2. Download the artifact bundle that matches your platform (e.g., `loqa-core_nightly-YYYYMMDD_linux_amd64.tar.gz`).
3. Verify integrity:

   ```bash
   shasum -a 256 -c loqa-core_nightly-YYYYMMDD_linux_amd64.tar.gz.sha256
   ```
4. Extract and add the binaries to your `PATH`:

   ```bash
   tar -xzf loqa-core_nightly-YYYYMMDD_linux_amd64.tar.gz
   export PATH="$PWD/loqa-core_nightly-YYYYMMDD_linux_amd64/bin:$PATH"
   ```

You can perform the same download with the GitHub CLI:

```bash
gh run download \
  --repo ambiware-labs/loqa-core \
  --workflow nightly.yml \
  --latest \
  --name nightly-YYYYMMDD-linux-amd64
```

### Option B: Build from source

```bash
git clone https://github.com/ambiware-labs/loqa-core.git
cd loqa-core
make skills            # compiles TinyGo examples and validates manifests
make run               # builds and launches the runtime with the example config
```

The `make run` target automatically rebuilds `loqad` with Go 1.24 toolchains and starts it using `config/example.yaml`.

## Step 2 – Start core dependencies

Loqa requires a NATS cluster with JetStream enabled. For development you can run a single instance:

```bash
docker run --rm -p 4222:4222 nats:2.10-alpine -js
```

Optional services that enhance the experience:

- **Ollama** (`ollama run llama3.2`) or a llama.cpp server for local LLM inference.
- **faster-whisper** (`python3 stt/faster_whisper.py`) for streaming STT.
- **Kokoro TTS** (`python3 tts/kokoro_stub.py`) for local speech synthesis.

## Step 3 – Configure the runtime

Copy the example configuration and adjust it for your environment:

```bash
cp config/example.yaml config/local.yaml
```

Key fields to review:

- `bus.servers`: Update to point at your NATS deployment.
- `skills.directory`: Directory containing skill manifests and WASM modules.
- `stt`, `llm`, `tts` blocks: Set `enabled: true` and update `mode` / `command` for real models.
- `telemetry`: Configure OTLP metrics/traces if you want to stream into Grafana/Tempo.

Environment variables override any value in the YAML file (see `README.md` for the full list). This makes it easy to script different environments without committing config changes.

## Step 4 – Launch and verify

Start the runtime with your config:

```bash
go run ./cmd/loqad --config ./config/local.yaml
```

Expect log lines similar to:

```
{"time":"...","level":"INFO","msg":"connected to NATS","servers":"nats://localhost:4222"}
{"time":"...","level":"INFO","msg":"runtime started","addr":"0.0.0.0:8080"}
```

Health endpoints are exposed on the configured HTTP port:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

Both endpoints return HTTP 200 when the runtime is operating normally.

## Step 5 – Enable the voice pipeline (optional)

1. Toggle the feature flags in `config/local.yaml`:

   ```yaml
   stt:
     enabled: true
     mode: exec
     command: "python3 stt/faster_whisper.py"
     model_path: ./models/ggml-base.bin
   llm:
     enabled: true
     mode: ollama
     endpoint: http://localhost:11434
   tts:
     enabled: true
     mode: exec
     command: "python3 tts/kokoro_stub.py"
     voice: en-US
   ```

2. Start the helper services in separate terminals:

   ```bash
   python3 stt/faster_whisper.py --model small.en
   ollama serve
   python3 tts/kokoro_stub.py
   ```

3. Publish an audio frame (or simulated command) on `audio.frame` to exercise the pipeline. During development you can send a text transcript directly to the router:

   ```bash
   nats pub stt.text.final '{"text":"set a five second tea timer"}'
   ```

Watch for subsequent messages on:

- `nlu.request` / `nlu.response.*` for LLM planning
- `tts.request` / `tts.audio` for synthesized speech frames
- `skill.timer.status` for the invoked skill

## Troubleshooting

| Symptom | Recommended action |
| --- | --- |
| `connect to nats: no servers available` | Confirm NATS is running and reachable; check firewall rules and `bus.servers`. |
| TinyGo cannot find Go toolchain | Reinstall TinyGo 0.39+. Ensure `tinygo env GOVERSION` reports 1.22 and not 1.24 (TinyGo pins its own toolchain). |
| `ollama: connection refused` | Start Ollama with `ollama serve` or adjust the endpoint to your inference host. |
| `tts publish blocked` warnings | Add the destination subject to the skill manifest `capabilities.bus.publish` list. |
| HTTP health check fails | Inspect runtime logs for initialization errors (skills failing to load, bus authentication issues, etc.). |

If you get stuck, open a Discussion on GitHub or drop a note in the Ambiware Labs community channel.
