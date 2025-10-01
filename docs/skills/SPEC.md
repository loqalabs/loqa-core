# Loqa Skill Specification (v1)

This document formalizes the manifest schema and host runtime API for first-party and third-party skills that run inside Loqa. It reflects the `host_version: v1` ABI shipped with `loqa-core` as of the MVP timeframe.

> **TL;DR**
> - Skills are packaged as WASM modules plus a `skill.yaml` manifest.
> - Manifests declare metadata, runtime requirements, capabilities, permissions, and optional surfaces.
> - The runtime exposes deterministic host functions and environment variables to the module; additional privileges must be explicitly requested via `permissions`.

## Semantic versioning & compatibility

- **Manifest schema:** Follow [SemVer](https://semver.org/). Providers SHOULD bump `metadata.version` for any change and MUST bump the major version when breaking inputs/outputs.
- **Host ABI:** The WASM API is versioned via `runtime.host_version`. Loqa currently supports `v1`. Future revisions will remain parallel so older skills continue to load until an explicit deprecation cycle completes.
- **Runtime compatibility:** `loqa-skill validate` enforces the schema statically. At runtime the host re-validates permissions and subject declarations before invoking the module.

## Manifest structure

```yaml
metadata:
  name: timer
  version: 0.1.0
  description: Simple countdown timer skill that announces when time elapses.
  author: Loqa Labs
  tags: [timers, voice]
runtime:
  mode: wasm
  module: build/timer.wasm
  entrypoint: run
  host_version: v1
capabilities:
  bus:
    subscribe:
      - skill.timer.start
      - skill.timer.cancel
    publish:
      - tts.request
      - skill.timer.status
  storage:
    kv: true
  timers: true
permissions:
  - bus:publish
  - bus:subscribe
  - event_store:read
surfaces:
  voice: true
  automations: true
```

### `metadata`

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `name` | string | ✅ | Must be globally unique within a deployment; kebab-case recommended. |
| `version` | string | ✅ | Semantic version string (e.g., `1.0.0`). |
| `description` | string | ✅ | Short human-readable summary. |
| `author` | string | ✅ | Maintainer name, org, or contact. |
| `tags` | string[] | optional | Keywords for discovery in registries/marketplace. |

### `runtime`

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `mode` | enum (`wasm`) | ✅ | Additional runtimes (native, exec) will ship after ABI review. |
| `module` | string | ✅ for `wasm` | Path to the WASM artifact relative to the manifest. |
| `entrypoint` | string | ✅ for `wasm` | Exported function invoked by the host. TinyGo defaults to `main`. |
| `host_version` | enum (`v1`) | ✅ | Declares the required host ABI. Future ABIs will use `v2`, etc. |

### `capabilities`

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `bus.publish` | string[] | conditional | Subjects the skill may publish to. Required if `permissions` include `bus:publish`. |
| `bus.subscribe` | string[] | conditional | Subjects the host should subscribe on the skill’s behalf. Required if the skill is event-driven. |
| `storage.kv` | bool | optional | Requests access to key/value storage APIs (planned). |
| `timers` | bool | optional | Marks that the skill schedules timers (used for observability & quotas). |

### `permissions`

Permissions gate host-side capabilities. The host enforces them per invocation.

| Permission | Grants |
| --- | --- |
| `bus:publish` | Ability to publish messages via `host.Publish`. Only subjects listed under `capabilities.bus.publish` are allowed. |
| `bus:subscribe` | Authority to listen on declared subscribe subjects (default for most event-driven skills). |
| `event_store:read` | Read access to the audit/event store (when specific APIs are exposed in future ABIs). |
| `http:call` | Permission to invoke outbound HTTP helpers (planned for `v2`). |

> Additional permissions may be introduced in future ABI revisions. Unknown permissions are ignored today but may cause validation failures once implemented—treat them as reserved words.

### `surfaces`

Optional hints describing which user-facing surfaces the skill targets. These do not grant capabilities but improve discovery.

| Key | Meaning |
| --- | --- |
| `voice` | Skill can be triggered via voice pipeline. |
| `display` | Skill surfaces information on visual displays or dashboards. |
| `automations` | Skill participates in scheduled/conditional workflows. |

## Host ABI v1

### Environment variables

The host populates the following environment variables for each invocation:

| Variable | Description |
| --- | --- |
| `LOQA_SKILL_NAME` | Skill identifier from `metadata.name`. |
| `LOQA_EVENT_SUBJECT` | NATS subject that triggered the invocation. |
| `LOQA_EVENT_PAYLOAD` | Raw message payload (UTF-8 JSON by convention). |
| `LOQA_EVENT_REPLY` | Reply subject (present only when the publisher requested a response). |
| `LOQA_INVOCATION_ID` | Unique UUID for tracing. |
| `LOQA_SKILL_DIRECTORY` | Absolute path to the skill’s directory on disk. |

### Imported host functions

Under `host_version: v1` TinyGo/WASM modules can import these functions (see `skills/examples/internal/host` helpers):

| Function | Signature | Description |
| --- | --- | --- |
| `env.host_log(ptr, len)` | `(i32, i32) -> ()` | Emits a log line captured in runtime logs and the audit trail. |
| `env.host_publish(subjectPtr, subjectLen, payloadPtr, payloadLen)` | `(i32, i32, i32, i32) -> i32` | Publishes payload to NATS subject. Returns `0` on success, non-zero on failure (e.g., permission denied). |

To use them from TinyGo, import the helper `skills/examples/internal/host` and call `host.Log` / `host.Publish`. Publishing will fail if the manifest omits `bus:publish` or the subject is not listed in `capabilities.bus.publish`.

### Audit events

The host records `skill.invoke.start`, `skill.invoke.error`, and `skill.invoke.complete` events (plus `skill.publish`) in the event store when available. Skills currently cannot write to the event store directly; future APIs will be gated by additional permissions.

## Validation workflow

- Run `go run ./cmd/loqa-skill validate --file skill.yaml` before packaging.
- CI uses the same validator, so manifests that break the spec will fail pull requests.
- The runtime re-validates and refuses to load skills missing required permissions or subjects.

## Roadmap & feedback

Open questions and future work are tracked in:

- [loqa-core#37](https://github.com/loqalabs/loqa-core/issues/37) – skills spec evolution
- [loqa-meta#26](https://github.com/loqalabs/loqa-meta/issues/26) – marketplace RFC
- [loqa-meta#27](https://github.com/loqalabs/loqa-meta/issues/27) – Extension Labs resources

If you need additional host capabilities, open a discussion or RFC in `loqa-meta`.
