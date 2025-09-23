# Observability Stack

Spin up Prometheus, Grafana, Tempo, and Loki locally to visualize Loqa runtime telemetry.

## Requirements
- Docker + docker-compose (v2)
- macOS users: `host.docker.internal` is used for Prometheus scraping. On Linux, update `observability/prometheus/prometheus.yml` to point to the host IP address instead.

## Usage
```bash
docker compose -f observability/docker-compose.yaml up -d
```

Then export the following before launching the runtime:
```bash
export LOQA_TELEMETRY_OTLP_ENDPOINT=localhost:4317
export LOQA_TELEMETRY_PROMETHEUS_BIND=:9091
```

Run the runtime in another terminal:
```bash
go run ./cmd/loqad --config ./config/example.yaml
```

### Accessing the stack
- Grafana: http://localhost:3000 (username/password: `admin`/`admin`)
- Prometheus: http://localhost:9090
- Tempo API: http://localhost:3200
- Loki API: http://localhost:3100

Provisioned dashboards and datasources live under the `Loqa` folder in Grafana. Update the dashboard JSONs inside `observability/grafana/dashboards/` as the system evolves.

Shutdown with:
```bash
docker compose -f observability/docker-compose.yaml down
```
