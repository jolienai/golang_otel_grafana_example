# Dashboard Guide

This project provisions the **Go API Tempo Traces** dashboard in Grafana:

```text
http://localhost:3000/d/go-api-tempo-traces/go-api-tempo-traces
```

Login:

```text
admin / admin
```

The dashboard combines three observability signals:

- `Prometheus` metrics from the API `/metrics` endpoint.
- `Tempo` traces exported by the Go API through OpenTelemetry.
- `Loki` logs collected from Docker container stdout by Promtail.

## Metrics Panels

### Request Rate by Route

Shows requests per second grouped by API route.

Why it matters:

- Confirms the API is receiving traffic.
- Shows which endpoints are used most.
- Helps correlate traffic spikes with latency, CPU, memory, and trace volume.

### p95 Request Latency

Shows the 95th percentile request duration by route.

Why it matters:

- Focuses on slower user-visible requests instead of only averages.
- Helps detect regressions when most requests are fine but a meaningful tail is slow.
- Useful for comparing `/hello`, `/work`, and `/healthz` behavior.

### Requests in Selected Range

Shows total requests per route in the dashboard time window.

Why it matters:

- Gives a quick volume summary for the selected period.
- Helps verify that a panel is empty because there was no traffic, not because scraping or tracing is broken.

### Synthetic Work Throughput

Shows completed `/work` steps per second.

Why it matters:

- Tracks the amount of synthetic work the API is doing.
- Helps correlate work volume with latency, CPU usage, memory usage, and traces.

### API CPU Usage

Shows CPU used by the Go API process.

Why it matters:

- Indicates whether the service is CPU-bound.
- Helps explain latency increases during bursts of traffic.
- Useful when comparing request volume with resource usage.

### API Memory Usage

Shows resident memory and Go heap allocation.

Why it matters:

- Resident memory shows the process memory footprint.
- Heap allocation shows memory actively managed by Go.
- Growth over time can indicate leaks, large buffers, or allocation-heavy code paths.

### Error Rate

Shows non-2xx request rate. It displays `0` when there are no errors.

Why it matters:

- Separates traffic volume from failing traffic.
- Helps spot broken routes, bad deploys, or dependency failures.
- Should normally stay at zero unless you call the simulated error endpoint.

Generate error data:

```bash
curl -i "http://localhost:8080/error"
```

### Go Goroutines

Shows the number of active goroutines in the Go process.

Why it matters:

- Goroutine growth can indicate leaked background work or stuck requests.
- Useful for spotting concurrency problems before they become memory or latency issues.

### Average GC Pause

Shows average Go garbage collection pause duration.

Why it matters:

- GC pauses can contribute to latency spikes.
- Rising pauses may indicate allocation pressure.
- Useful when viewed alongside p95 latency, memory usage, and request rate.

### API Uptime

Shows how long the API process has been running.

Why it matters:

- Confirms whether the API restarted.
- Helps explain sudden drops in metrics or traces.
- Useful when checking whether changes were applied after a rebuild.

### In-flight Requests

Shows currently active HTTP requests.

Why it matters:

- Indicates concurrent load.
- Helps identify stuck or slow requests when the value stays elevated.
- Useful when compared with request latency and CPU usage.

## Log Panels

### Recent API Logs

Shows structured JSON log messages emitted by the Go API.

Why it matters:

- Shows handler-level messages without leaving Grafana.
- Helps explain what happened inside a request when metrics show an error or latency spike.
- Complements traces with application messages, such as request completion, synthetic work steps, and simulated errors.

The panel uses this Loki query:

```logql
{compose_service="api"}
```

You can also inspect logs in Grafana Explore by selecting the `Loki` datasource and running the same query.

## Trace Panels

### Recent API Traces

Shows recent Tempo traces for the `golang-otel-api` service.

Why it matters:

- Lets you inspect individual requests end to end.
- Useful when a metric shows a spike and you need to see what happened inside a request.

### Work Endpoint Traces

Shows traces containing the `/work` synthetic work span.

Why it matters:

- Breaks down each work request into nested spans.
- Shows how the simulated work steps contribute to total request time.

### Greeting Traces

Shows traces for the `/hello` greeting path.

Why it matters:

- Demonstrates a smaller traced workflow.
- Useful for comparing a simple endpoint against the heavier `/work` endpoint.

## How to Generate Data

Run this from the project root while the stack is running:

```bash
for i in 1 2 3 4 5 6 7 8 9 10; do
  curl -s "http://localhost:8080/work?steps=5" > /dev/null
  curl -s "http://localhost:8080/hello?name=Jolien" > /dev/null
done
```

Generate a simulated 5xx response:

```bash
curl -i "http://localhost:8080/error"
```

Prometheus scrapes every `5s`, so wait a few seconds before expecting new graph samples.
