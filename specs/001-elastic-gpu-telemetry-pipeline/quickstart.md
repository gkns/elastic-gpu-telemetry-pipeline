# Quickstart: Elastic GPU Telemetry Pipeline

**Date**: 2026-05-06

---

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.22+ | Build all components |
| Docker | 24+ | Run QuestDB and component containers |
| Docker Compose | v2 | Local multi-service orchestration |
| KIND | 0.23+ | Optional — Kubernetes cluster testing |
| swag CLI | latest | OpenAPI generation (`go install github.com/swaggo/swag/cmd/swag@latest`) |

---

## Repository Layout

```text
cmd/
├── mq/            # Message Queue server
├── streamer/      # Telemetry Streamer (CSV → MQ)
├── collector/     # Telemetry Collector (MQ → QuestDB)
└── api-gateway/   # REST API Gateway
internal/
├── protocol/      # Binary frame codec
├── mq/            # MQ router, buffer, in-flight tracker, WAL
├── telemetry/     # CSV parser, TelemetryRecord
├── questdb/       # ILP writer (Collector), pgx client (API Gateway)
└── config/        # Per-component config from env vars
deploy/
├── docker/        # One Dockerfile per component
├── compose/       # docker-compose.yml
└── k8s/           # KIND manifests
data/
└── input_dcgm_metrics_20250718_134233.csv
Makefile
```

---

## Option A: Run Locally (Go binary + Docker QuestDB)

### Step 1 — Start QuestDB

```bash
docker run -d --name questdb \
  -p 8812:8812 \   # PostgreSQL wire (API Gateway queries)
  -p 9009:9009 \   # ILP (Collector writes)
  -p 9000:9000 \   # QuestDB Web Console (optional)
  questdb/questdb:latest
```

### Step 2 — Build all binaries

```bash
make build
# Produces: bin/mq, bin/streamer, bin/collector, bin/api-gateway
```

### Step 3 — Start the Message Queue

```bash
./bin/mq
# Default: listens on :7000
# Env overrides: MQ_LISTEN_ADDR, MQ_WAL_DIR, MQ_SNAPSHOT_INTERVAL
```

### Step 4 — Start a Collector

```bash
./bin/collector
# Default: connects to localhost:7000 (MQ), writes to localhost:9009 (QuestDB ILP)
# Env overrides: COLLECTOR_MQ_ADDR, COLLECTOR_QUESTDB_ILP_ADDR, COLLECTOR_ID
```

### Step 5 — Start the API Gateway

```bash
./bin/api-gateway
# Default: listens on :8080, queries localhost:8812 (QuestDB pgx)
# Env overrides: GATEWAY_LISTEN_ADDR, GATEWAY_QUESTDB_PG_ADDR
```

### Step 6 — Start a Streamer

```bash
./bin/streamer --csv data/input_dcgm_metrics_20250718_134233.csv
# Default: connects to localhost:7000 (MQ)
# Env overrides: STREAMER_MQ_ADDR, STREAMER_ID, STREAMER_SEND_RATE
```

### Step 7 — Query the API

```bash
# List GPUs
curl http://localhost:8080/api/v1/gpus

# Get telemetry for a GPU (replace UUID)
curl "http://localhost:8080/api/v1/gpus/GPU-5fd4f087-86f3-7a43-b711-4771313afc50/telemetry?page=1&page_size=20"

# With time range
curl "http://localhost:8080/api/v1/gpus/GPU-5fd4f087-86f3-7a43-b711-4771313afc50/telemetry?start_time=2025-07-18T20:00:00Z&end_time=2025-07-18T21:00:00Z"
```

---

## Option B: Docker Compose (all services)

```bash
cd deploy/compose
docker compose up
```

This starts: `questdb`, `mq`, `collector` (1 replica), `streamer` (1 replica), `api-gateway`.

Scale collectors and streamers:

```bash
docker compose up --scale collector=3 --scale streamer=2
```

---

## Option C: KIND Cluster

```bash
# Create cluster
kind create cluster --config deploy/k8s/kind-config.yaml

# Deploy all components
kubectl apply -f deploy/k8s/

# Port-forward API Gateway
kubectl port-forward svc/api-gateway 8080:8080
```

---

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build all binaries to `bin/` |
| `make test` | Run unit tests with coverage report |
| `make test-integration` | Run integration tests (requires Docker) |
| `make swagger` | Regenerate OpenAPI spec in `specs/001-elastic-gpu-telemetry-pipeline/contracts/` |
| `make docker-build` | Build Docker images for all components |
| `make lint` | Run `golangci-lint` |
| `make clean` | Remove `bin/` and generated files |

---

## Environment Variables Reference

### MQ

| Variable | Default | Description |
|----------|---------|-------------|
| `MQ_LISTEN_ADDR` | `:7000` | TCP listen address |
| `MQ_WAL_DIR` | `./wal` | WAL directory for snapshots |
| `MQ_SNAPSHOT_INTERVAL` | `30s` | How often to write WAL checkpoint |
| `MQ_INFLIGHT_TIMEOUT` | `5s` | ACK timeout before redelivery |
| `MQ_MAX_RETRIES` | `3` | Max redelivery attempts |
| `MQ_BUFFER_CAP` | `1000` | Per-streamer channel capacity |

### Streamer

| Variable | Default | Description |
|----------|---------|-------------|
| `STREAMER_MQ_ADDR` | `localhost:7000` | MQ address |
| `STREAMER_ID` | hostname | Streamer identifier |
| `STREAMER_CSV_PATH` | (required) | Path to CSV telemetry file |
| `STREAMER_SEND_RATE` | `0` | Delay between sends (e.g., `10ms`); 0 = max speed |

### Collector

| Variable | Default | Description |
|----------|---------|-------------|
| `COLLECTOR_MQ_ADDR` | `localhost:7000` | MQ address |
| `COLLECTOR_ID` | hostname | Collector identifier |
| `COLLECTOR_QUESTDB_ILP_ADDR` | `localhost:9009` | QuestDB ILP write endpoint |
| `COLLECTOR_QUESTDB_PG_ADDR` | `localhost:8812` | QuestDB pgx (for DDL at startup) |
| `COLLECTOR_WORKERS` | `4` | Parallel ILP writer goroutines |

### API Gateway

| Variable | Default | Description |
|----------|---------|-------------|
| `GATEWAY_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `GATEWAY_QUESTDB_PG_ADDR` | `localhost:8812` | QuestDB pgx query endpoint |
| `GATEWAY_PAGE_SIZE_MAX` | `1000` | Maximum allowed page_size |

---

## Observability

- **MQ**: logs connection events, routing decisions, redeliveries to stderr (structured JSON).
- **Streamer**: logs rows sent, ACKs received, NACK backoffs.
- **Collector**: logs messages received, ILP write latency, QuestDB errors.
- **API Gateway**: logs request method/path/status/duration per request.
- **QuestDB Web Console**: http://localhost:9000 — run ad-hoc SQL queries.

---

## Running Tests

```bash
# Unit tests only (no external dependencies)
make test

# Integration tests (starts QuestDB and MQ via testcontainers or Docker Compose)
make test-integration
```

Coverage reports are written to `coverage.html`. Minimum coverage threshold: **70% per package**.
