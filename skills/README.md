# Skills

Loqa skills are packaged as WASM modules accompanied by a `skill.yaml` manifest that advertises metadata, runtime configuration, capabilities, and permissions. The examples in `skills/examples` act as reference implementations and regression fixtures for the host runtime.

## Prerequisites

- Go toolchain for running the manifest validator (`go run ./cmd/loqa-skill ...`).
- [TinyGo](https://tinygo.org/) `0.39+` to compile the WASM modules targeting WASI.
- (Optional) A WASI runtime such as `wasmtime` if you want to execute the modules in isolation.

## Build workflow

Each example ships with TinyGo sources under `src/` and a `skill.yaml` manifest. Compile and validate with:

```bash
cd skills/examples/timer
mkdir -p build
tinygo build -o build/timer.wasm -target=wasi ./src

go run ./cmd/loqa-skill validate --file skill.yaml
```

Repeat the same steps for other examples like `smart-home`.

> CI note: The GitHub Actions workflow automatically builds these references with TinyGo and validates each `skill.yaml`, so keep the manifests and TinyGo sources in sync with your changes.

## Local testing

Both examples accept environment variables so you can simulate inbound events before integrating with Loqa:

- `LOQA_TIMER_REQUEST` drives the timer countdown example.
- `LOQA_SMART_HOME_INTENT` plus optional Home Assistant credentials drive the smart-home bridge.

See the README inside each example directory for precise commands.

## Deploying skills into Loqa

1. Copy the compiled WASM binaries and manifests into the runtime's skill directory.
2. Ensure the runtime has been granted the permissions declared in the manifest.
3. Restart or hot-reload the runtime so the new skills are discovered.
4. Publish the subjects listed under `capabilities.bus` to exercise the skills once the host wiring is complete.
