# Smart Home Bridge Skill

Mock bridge that demonstrates how a WASM skill could forward intents to a Home Assistant deployment. The manifest highlights required message subjects, persistent storage, and host permissions for outbound HTTP requests.

## Build (TinyGo)

```bash
cd skills/examples/smart-home
mkdir -p build
tinygo build -o build/smart-home.wasm -target=wasi ./src
```

## Local smoke test

The module reads configuration from environment variables so you can experiment without a Loqa host:

```bash
wasmtime run \
  --env HOMEASSISTANT_URL="http://localhost:8123" \
  --env HOMEASSISTANT_TOKEN="demo-token" \
  --env LOQA_SMART_HOME_INTENT='{"room":"kitchen","device":"light.kitchen","action":"turn_on","payload":"brightness=80"}' \
  build/smart-home.wasm
```

It does not perform real HTTP calls yet; instead it logs the request that would be sent via the host runtime.

## Deploying into Loqa

1. Validate the manifest:
   ```bash
   go run ./cmd/loqa-skill validate --file skill.yaml
   ```
2. Copy both `skill.yaml` and `build/smart-home.wasm` into the configured skills directory.
3. Ensure the runtime grants the `http:call`, `bus:publish`, and `bus:subscribe` permissions defined in the manifest.
