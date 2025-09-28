# Loqa Skill Authoring Guide

This guide walks through building, packaging, and registering third-party skills for the Loqa runtime. For a formal description of the manifest schema and host ABI, see [`docs/skills/SPEC.md`](../docs/skills/SPEC.md).

## 1. Project layout

Each skill lives in its own directory containing:

- `skill.yaml` – manifest describing metadata, runtime module path, capabilities, and permissions.
- `src/` – TinyGo sources compiled to a WASI-compatible WASM module.
- Optional assets or configuration files referenced by the manifest (for example, static prompts).

Example tree:

```
skills/
  my-skill/
    skill.yaml
    src/
      main.go
```

## 2. Manifest essentials

`skill.yaml` is validated with `go run ./cmd/loqa-skill validate --file skill.yaml`. Required sections:

- `metadata` – name, version, description, and author.
- `runtime` – currently `mode: wasm`, relative path to the compiled module, entrypoint function, and host ABI version (`v1`).
- `capabilities.bus.publish` and `.subscribe` – NATS subjects this skill will interact with.
- `permissions` – opt-in host powers such as `bus:publish` or `event_store:read`.

The runtime enforces both permissions and declared subjects at execution time; attempts to publish a subject not listed in the manifest are rejected.

## 3. Building the WASM module

- Install TinyGo `0.39.0` or newer.
- Build with the WASI target:

  ```bash
  tinygo build -o build/my-skill.wasm -target=wasi ./src
  ```

  The CI pipeline uses the same command, so keep sources and manifests in sync.

## 4. Runtime environment variables

The runtime injects context for each invocation via environment variables:

| Variable | Description |
|----------|-------------|
| `LOQA_EVENT_SUBJECT` | NATS subject that triggered the invocation |
| `LOQA_EVENT_PAYLOAD` | Raw payload bytes (UTF-8 JSON by convention) |
| `LOQA_EVENT_REPLY` | Optional reply subject |
| `LOQA_INVOCATION_ID` | Unique identifier for the invocation |
| `LOQA_SKILL_DIRECTORY` | Absolute path to the skill directory |

Use these to deserialize requests and locate assets.

## 5. Host APIs

The shared helper in `skills/examples/internal/host` exposes:

- `host.Log(string)` – writes to runtime logs and the audit trail.
- `host.Publish(subject string, payload []byte) bool` – publishes to NATS (requires `bus:publish` and a declared subject).

Future ABI versions will add storage and HTTP helpers; design manifests with explicit permissions so skills remain sandboxed.

## 6. Deployment steps

1. Compile the WASM module and validate the manifest.
2. Copy both `skill.yaml` and the compiled module into the runtime's skill directory (default `./skills`).
3. Restart or hot-reload the runtime. The skills service watches the directory on startup and subscribes to declared subjects.
4. Emit test events on the declared subjects to verify behaviour. Watch telemetry and the event store audit log for activity.

## 7. Observability & audit

Each invocation generates audit events in the local event store, including start, completion, and publish activity. Use these logs to troubleshoot permission failures or unexpected outcomes.

## 8. Next steps

- Review the reference implementations in `skills/examples` for patterns.
- Contribute new capabilities via pull requests or RFCs in `loqa-meta`.
- Share reusable skills with the community and tag manifests with descriptive metadata for discovery.

Happy building!
