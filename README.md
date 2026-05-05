# golang_otel_grafana_example

Practical minimal setup for a Go HTTP API with OpenTelemetry traces exported to Grafana Tempo, Prometheus metrics, Loki logs collected by Promtail, and Grafana dashboards.

## Run

```bash
docker compose up --build
```

Services:

- API: http://localhost:8080
- Grafana: http://localhost:3000 (`admin` / `admin`)
- Tempo: http://localhost:3200
- Prometheus: http://localhost:9090
- Loki: http://localhost:3100
- Dashboard: http://localhost:3000/d/go-api-tempo-traces/go-api-tempo-traces

## Generate Data

```bash
curl "http://localhost:8080/healthz"
curl "http://localhost:8080/hello?name=Jolien"
curl "http://localhost:8080/work?steps=5"
curl "http://localhost:8080/error"
```

Generate a burst of traffic so the graph panels have visible lines:

```bash
for i in 1 2 3 4 5 6 7 8 9 10; do
  curl -s "http://localhost:8080/work?steps=5" > /dev/null
  curl -s "http://localhost:8080/hello?name=Jolien" > /dev/null
done
```

Generate a simulated 5xx error so the `Error Rate` panel has data:

```bash
curl -i "http://localhost:8080/error"
```

Run a k6 load test with 100 virtual users calling `/work?steps=5`:

```bash
k6 run k6/work-load.js
```

Override the API URL if needed:

```bash
BASE_URL=http://localhost:8080 k6 run k6/work-load.js
```

Open Grafana at http://localhost:3000 and use the provisioned **Go API Tempo Traces** dashboard. It includes Prometheus graphs for request rate, latency, errors, CPU, memory, goroutines, GC pause, uptime, and in-flight requests, a Loki log panel, plus Tempo trace tables.

## Logs

The API writes JSON logs to stdout. Promtail discovers Docker Compose containers through the Docker socket and sends those logs to Loki.

In Grafana, you can see logs in two places:

- Dashboard panel: `Recent API Logs`
- Explore: choose the `Loki` datasource and run this query:

```logql
{compose_service="api"}
```

Useful examples:

```logql
{compose_service="api"} |= "simulated server error"
{compose_service="api"} |= "request completed"
```

## Dashboard

For detailed panel-by-panel explanations, see [DASHBOARD.md](./DASHBOARD.md).

The Grafana dashboard is provisioned automatically from `observability/grafana/dashboards/go-otel-tempo.json` and is available at:

```text
http://localhost:3000/d/go-api-tempo-traces/go-api-tempo-traces
```

Login with:

```text
admin / admin
```

Datasources:

- `Prometheus` is used for metrics from the API `/metrics` endpoint.
- `Tempo` is used for distributed traces exported through OpenTelemetry OTLP.
- `Loki` is used for API logs collected by Promtail from Docker container stdout.

Dashboard panels:

- `Request Rate by Route` shows requests per second grouped by route.
- `p95 Request Latency` shows tail latency per route from Prometheus histograms.
- `Requests in Selected Range` shows total requests for the selected dashboard time window.
- `Synthetic Work Throughput` shows completed `/work` steps per second.
- `API CPU Usage` shows process CPU usage for the Go API.
- `API Memory Usage` shows resident memory and Go heap allocation.
- `Error Rate` shows non-2xx request rate and displays `0` when there are no errors.
- `Go Goroutines` shows active goroutine count.
- `Average GC Pause` shows average Go garbage collection pause duration.
- `API Uptime` shows how long the API process has been running.
- `In-flight Requests` shows currently active HTTP requests.
- `Recent API Logs` shows structured log messages from the API container.
- `Recent API Traces`, `Work Endpoint Traces`, and `Greeting Traces` show Tempo trace search results.

If the chart panels look empty, generate traffic and wait at least one Prometheus scrape interval. The scrape interval is `5s` in `observability/prometheus/prometheus.yml`.

## Endpoints

- `GET /healthz` returns service health.
- `GET /hello?name=<name>` returns a greeting and creates a nested trace span.
- `GET /work?steps=<1-10>` simulates multi-step work with one span per step.
- `GET /error` returns a simulated `500 Internal Server Error`.
- `GET /metrics` exposes Prometheus metrics.

## Local Development

```bash
go mod tidy
go run ./src
```

When running outside Docker, point the exporter to local Tempo:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 go run ./src
```
