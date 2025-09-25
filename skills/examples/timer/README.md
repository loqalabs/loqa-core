# Timer Skill

Reference countdown timer implemented as a WASM module. The skill listens for `skill.timer.start` and `skill.timer.cancel` subjects and publishes status updates and TTS prompts.

## Build (TinyGo)

```bash
cd skills/examples/timer
mkdir -p build
tinygo build -o build/timer.wasm -target=wasi ./src
```

## Local smoke test

The module logs through the host via `host_log`. You can emulate a timer request by passing JSON in the `LOQA_TIMER_REQUEST` environment variable when running the module with a WASI runtime (for example, `wasmtime`):

```bash
wasmtime run --env LOQA_TIMER_REQUEST='{"duration_ms":2000,"label":"tea"}' build/timer.wasm
```

The TinyGo entrypoint will pause for the requested duration and log start/complete messages.

## Deploying into Loqa

1. Validate the manifest:
   ```bash
   go run ./cmd/loqa-skill validate --file skill.yaml
   ```
2. Copy `skill.yaml` and the compiled `build/timer.wasm` into the skill search path configured for the runtime.
3. Restart or reload the runtime so it discovers the updated artifacts.
