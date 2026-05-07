# Elastic GPU Telemetry Pipeline

A Go monorepo that ingests DCGM GPU metrics from CSV files, routes them through a custom TCP message queue, persists them in QuestDB, and exposes a paginated REST API — all without any external message-broker dependency.

---

## Table of Contents

1. [Architecture](#architecture)
2. [Design Decisions](#design-decisions)
3. [Repository Layout](#repository-layout)
4. [Prerequisites](#prerequisites)
5. [Development — Running Individual Services](#development--running-individual-services)
6. [Docker Compose](#docker-compose)
7. [Helm Chart Deployment](#helm-chart-deployment)
8. [API Reference](#api-reference)
9. [Configuration Reference](#configuration-reference)
10. [Running Tests](#running-tests)

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│  Data Source                                                         │
│  CSV: input_dcgm_metrics_20250718_134233.csv                        │
│  (NVIDIA H100 DCGM metrics — GPU util, mem clock, temp, …)          │
└───────────────────────────┬──────────────────────────────────────────┘
                            │ TCP :7000  (HELLO → PUBLISH → ACK/NACK)
                            ▼
┌───────────────────────────────────────────────────────────┐
│  Streamer  (cmd/streamer)                                 │
│  • Reads CSV row-by-row, stamps ProcessedAt = time.Now()  │
│  • JSON-encodes each row as TelemetryRecord               │
│  • Wraps in PUBLISH frame with monotonic SeqNum           │
│  • Retries on NACK; exponential backoff reconnect         │
└───────────────────────────┬───────────────────────────────┘
                            │ Binary TCP frames
                            ▼
┌───────────────────────────────────────────────────────────┐
│  Message Queue  (cmd/mq)                                  │
│  • Custom binary protocol: 13-byte header                 │
│    [1B type | 4B length | 8B msgID]                       │
│  • HELLO handshake identifies Streamers vs Collectors     │
│  • Sticky FNV-1a routing: hash(streamerID) mod N          │
│    → preserves per-streamer ordering across collectors    │
│  • In-flight tracker: 5 s timeout, 3 retries, redelivery │
│  • WAL: append-only per-streamer .wal files + checkpoint  │
│  • Back-pressure: blocks TCP read when buffer full        │
│    (does NOT NACK — avoids amplifying retries)            │
│  • 30 s read deadline per connection                      │
└──────────┬──────────────────────────────┬─────────────────┘
           │ DELIVER frames               │ DELIVER frames
           ▼                              ▼
┌──────────────────┐            ┌──────────────────┐
│  Collector  [1]  │            │  Collector  [2]  │
│  (cmd/collector) │            │  (cmd/collector) │
│  • ILP writer    │            │  • ILP writer    │
│  • Worker pool   │            │  • Worker pool   │
│  • 3× retry +    │            │  • 3× retry +    │
│    backoff write │            │    backoff write  │
│  • ACK/NACK back │            │  • ACK/NACK back │
└──────────┬───────┘            └────────┬─────────┘
           │ ILP :9009                   │ ILP :9009
           └──────────────┬──────────────┘
                          ▼
             ┌────────────────────────┐
             │  QuestDB               │
             │  gpu_telemetry table   │
             │  PARTITION BY DAY WAL  │
             │  :9009 ILP writes      │
             │  :8812 pgwire queries  │
             └────────────┬───────────┘
                          │ pgwire :8812
                          ▼
             ┌────────────────────────┐
             │  API Gateway           │
             │  (cmd/api-gateway)     │
             │  GET /api/v1/gpus      │
             │  GET /api/v1/gpus/{id} │
             │      /telemetry        │
             │  GET /healthz          │
             │  chi router + pgx/v5   │
             └────────────────────────┘
```

### Message Flow (happy path)

```
Streamer                  MQ                     Collector
   │  HELLO(streamer-1)   │                          │
   │─────────────────────►│                          │
   │  ACK(msgID=0)        │  HELLO(collector-1)      │
   │◄─────────────────────│◄─────────────────────────│
   │                      │  ACK(msgID=0)            │
   │                      │─────────────────────────►│
   │  PUBLISH(seq=1,body) │                          │
   │─────────────────────►│                          │
   │  ACK(seq=1)          │  DELIVER(msgID,body)     │
   │◄─────────────────────│─────────────────────────►│
   │                      │                ACK(msgID)│
   │                      │◄─────────────────────────│
```

---

## Design Decisions

### 1. Custom binary TCP protocol instead of an existing broker

Using Kafka, NATS, or RabbitMQ would add operational weight (cluster management, schema registry, ACL configuration). The pipeline's requirements are narrow: ordered delivery per-streamer, at-least-once guarantees, and horizontal collector scaling. A custom 13-byte binary framing (type + length + msgID) gives full control over back-pressure and routing semantics at ~zero overhead.

### 2. Sticky FNV-1a routing

`FNV1a(streamerID) mod len(collectors)` is used instead of round-robin. This guarantees that all messages from a given streamer always go to the same collector, preserving per-streamer ordering in QuestDB and avoiding race conditions on the same `gpu_id` across multiple ILP connections. When a collector disconnects, un-ACKed in-flight messages are rehashed to the remaining collectors and redelivered.

### 3. WAL + checkpoint for MQ durability

Each streamer gets its own `.wal` append-only file. A JSON checkpoint records the last-committed WAL offset for each streamer every 30 seconds. On restart, the MQ replays WAL entries since the last checkpoint, re-enqueuing un-ACKed messages. This gives durable at-least-once delivery without an external state store.

### 4. Back-pressure via TCP read blocking (not NACK)

When the per-streamer in-memory buffer is full, the MQ pauses reading from the streamer's TCP connection instead of sending a NACK. Sending a NACK would cause the streamer to retry immediately, amplifying congestion. Blocking the TCP read propagates back-pressure up to the streamer's kernel socket buffer, which is the standard TCP flow-control mechanism.

### 5. ProcessedAt timestamp injection

The CSV `timestamp` column is intentionally ignored. `ProcessedAt = time.Now()` is injected by the Streamer at publish time. This timestamps when the metric entered the pipeline, not when it was recorded on the GPU host — which is useful for latency analysis and avoids clock skew from remote hosts.

### 6. QuestDB ILP + pgwire split

The Collector writes via ILP (InfluxDB Line Protocol, port 9009) — a fire-and-forget append protocol optimized for high-throughput ingestion. The API Gateway queries via the PostgreSQL wire protocol (port 8812) using `pgx/v5`. This separates the write path (throughput-optimized) from the read path (query-optimized) and avoids mixing concerns in a single connection pool.

### 7. Collector uses `metadata.name` as its ID

In Kubernetes, each Collector pod gets a unique ordinal name (e.g., `collector-0`, `collector-1`). Using `metadata.name` as `COLLECTOR_ID` ensures stable, unique IDs across restarts, which makes WAL redelivery and router state deterministic.

---

## Repository Layout

```
cmd/
├── mq/             # Message Queue server binary
├── streamer/       # Telemetry Streamer binary (CSV → MQ)
├── collector/      # Telemetry Collector binary (MQ → QuestDB)
└── api-gateway/    # REST API Gateway binary (QuestDB → HTTP)
internal/
├── protocol/       # Binary frame codec (ReadFrame / WriteFrame)
├── mq/             # Router, StreamerQueue, InFlightTracker, WAL
├── telemetry/      # TelemetryRecord struct, CSV parser
├── questdb/        # ILP writer, pgx pool client, DDL
└── config/         # Per-component env-var configuration
deploy/
├── docker/         # Multi-stage Dockerfiles (one per binary)
├── compose/        # docker-compose.yml
├── k8s/            # Raw Kubernetes manifests + KIND config
└── helm/           # Helm chart (elastic-gpu-telemetry/)
specs/
└── 001-elastic-gpu-telemetry-pipeline/
    ├── spec.md         # Functional requirements & user stories
    ├── plan.md         # Architecture & tech stack
    ├── tasks.md        # Completed implementation task list
    ├── data-model.md   # Wire protocol, DB schema, config structs
    ├── research.md     # Routing algorithm rationale
    └── contracts/      # OpenAPI (swagger.json / swagger.yaml)
data/
└── input_dcgm_metrics_20250718_134233.csv   # Sample DCGM metrics
Makefile
go.mod
```

---

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.23+ | Build all binaries |
| Docker | 24+ | Run QuestDB; build images |
| Docker Compose | v2 | Local multi-service stack |
| Helm | 3.14+ | Kubernetes deployment |
| KIND | 0.23+ | Local Kubernetes cluster |
| kubectl | 1.29+ | Manage Kubernetes resources |

Install the `swag` CLI for OpenAPI generation (optional):

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

---

## Development — Running Individual Services

Build all binaries first:

```bash
make build
# → bin/mq  bin/streamer  bin/collector  bin/api-gateway
```

### 1. Start QuestDB

```bash
docker run -d --name questdb \
  -p 8812:8812 \
  -p 9009:9009 \
  -p 9000:9000 \
  questdb/questdb:latest
```

The QuestDB web console is available at http://localhost:9000.

> **Note:** Port 7000 may be used by macOS AirPlay/ControlCenter. If so, set  
> `MQ_LISTEN_ADDR=:17000` and point Streamer/Collector at `localhost:17000`.

### 2. Start the Message Queue

```bash
MQ_LISTEN_ADDR=:17000 \
MQ_WAL_DIR=/tmp/mq-wal \
./bin/mq
```

Defaults: `MQ_LISTEN_ADDR=:7000`, `MQ_WAL_DIR=./wal`.

### 3. Start a Collector

```bash
COLLECTOR_MQ_ADDR=localhost:17000 \
./bin/collector
```

Start multiple collectors in separate terminals to exercise sticky routing:

```bash
# Terminal A
COLLECTOR_MQ_ADDR=localhost:17000 COLLECTOR_ID=c1 ./bin/collector

# Terminal B
COLLECTOR_MQ_ADDR=localhost:17000 COLLECTOR_ID=c2 ./bin/collector
```

### 4. Start the API Gateway

```bash
./bin/api-gateway
# Listens on :8081, connects to QuestDB at localhost:8812
```

### 5. Start the Streamer

```bash
STREAMER_MQ_ADDR=localhost:17000 \
STREAMER_CSV_PATH=data/input_dcgm_metrics_20250718_134233.csv \
./bin/streamer
```

The Streamer logs `streamer done rows_sent=2470` when the CSV is exhausted.

### 6. Query the API

```bash
# List GPUs
curl http://localhost:8081/api/v1/gpus

# Paginated telemetry for GPU 0
curl "http://localhost:8081/api/v1/gpus/0/telemetry?page=1&page_size=20"

# Time-range query
curl "http://localhost:8081/api/v1/gpus/0/telemetry?start_time=2025-07-18T20:00:00Z&end_time=2025-07-18T21:00:00Z"

# Health check
curl http://localhost:8081/healthz

# Swagger UI
open http://localhost:8081/swagger/index.html
```

### 7. Generate OpenAPI docs

```bash
make swagger
# → cmd/api-gateway/docs/swagger.json  (and docs.go, swagger.yaml)
# UI auto-served at /swagger/index.html (no restart needed after regen)
```

---

## Docker Compose

Runs the full pipeline (QuestDB + MQ + Streamer + Collector + API Gateway) with a single command.

```bash
# Build images
make docker-build

# Build and push to a registry (optional — for remote deployments)
make docker-push REGISTRY=myorg TAG=v1.0.0

# Start everything
docker compose -f deploy/compose/docker-compose.yml up

# Scale collectors and streamers
docker compose -f deploy/compose/docker-compose.yml up \
  --scale collector=3 \
  --scale streamer=2
```

Services are available at:

| Service | URL |
|---------|-----|
| API Gateway | http://localhost:8081 |
| Swagger UI | http://localhost:8081/swagger/index.html |
| QuestDB Web Console | http://localhost:9000 |
| QuestDB pgwire | localhost:8812 |
| MQ TCP | localhost:7000 |

---

## Helm Chart Deployment

### Build and push images

The chart references images by the names in `values.yaml`.

**Local KIND cluster** — build and load images directly into the cluster:

```bash
make docker-build

kind load docker-image elastic-gpu-telemetry/mq:latest         --name gpu-telemetry
kind load docker-image elastic-gpu-telemetry/streamer:latest    --name gpu-telemetry
kind load docker-image elastic-gpu-telemetry/collector:latest   --name gpu-telemetry
kind load docker-image elastic-gpu-telemetry/api-gateway:latest --name gpu-telemetry
```

**Remote registry** — build, tag, and push in one step, then point the chart at the registry:

```bash
# Docker Hub
make docker-push REGISTRY=myorg TAG=v1.0.0

# GitHub Container Registry
make docker-push REGISTRY=ghcr.io/myorg TAG=v1.0.0
# The command I use: 
make docker-push REGISTRY=ghcr.io/gkns TAG=v1.0.0

# AWS ECR
make docker-push REGISTRY=123456789.dkr.ecr.us-east-1.amazonaws.com/gpu-telemetry TAG=v1.0.0
```

Then install the chart referencing the pushed images:

```bash
helm install elastic-gpu-telemetry deploy/helm/elastic-gpu-telemetry/ \
  --set imagePullPolicy=Always \
  --set mq.image=ghcr.io/gkns/mq:v1.0.0 \
  --set streamer.image=ghcr.io/gkns/streamer:v1.0.0 \
  --set collector.image=ghcr.io/gkns/collector:v1.0.0 \
  --set apiGateway.image=ghcr.io/gkns/api-gateway:v1.0.0
```

Or set a private registry pull secret:

```bash
helm install elastic-gpu-telemetry deploy/helm/elastic-gpu-telemetry/ \
  --set imagePullSecrets[0].name=regcred \
  --set imagePullPolicy=Always
```

### Create a KIND cluster

```bash
kind create cluster --name gpu-telemetry --config deploy/k8s/kind-config.yaml
```

> **macOS / KIND note:** KIND nodes are Docker containers whose internal IPs
> (e.g. `172.18.0.x`) are not reachable from the host. `kind-config.yaml`
> adds an `extraPortMappings` entry that forwards host port **30081** →
> NodePort **30081** on the control-plane node, making the API Gateway
> reachable at `http://localhost:30081` without port-forward.
> If you have an existing cluster without this mapping, recreate it or use
> `kubectl port-forward` instead.

### Install the chart

```bash
helm install elastic-gpu-telemetry deploy/helm/elastic-gpu-telemetry/ \
  --set imagePullPolicy=Never \
  --set apiGateway.service.type=NodePort
```

The CSV data is bundled inside the Streamer image — the Streamer pod starts reading automatically once MQ is healthy, no manual data loading required.

### Access the API Gateway

```bash
# NodePort — works when cluster was created with kind-config.yaml (extraPortMappings)
curl http://localhost:30081/api/v1/gpus
open http://localhost:30081/swagger/index.html

# port-forward — always works, including on existing clusters without extraPortMappings
kubectl port-forward svc/elastic-gpu-telemetry-api-gateway 8081:8081
curl http://localhost:8081/api/v1/gpus
open http://localhost:8081/swagger/index.html
```

### Scale collectors

```bash
helm upgrade elastic-gpu-telemetry deploy/helm/elastic-gpu-telemetry/ \
  --set collector.replicas=4
```

Because of sticky FNV-1a routing, increasing collector replicas distributes different streamer IDs across additional pods. Un-ACKed in-flight messages from the previous collector set are redelivered automatically.

### Access the QuestDB Web Console

QuestDB uses a ClusterIP service, so port-forward is always required:

```bash
kubectl port-forward svc/elastic-gpu-telemetry-questdb 9000:9000
open http://localhost:9000
```

### Uninstall

```bash
helm uninstall elastic-gpu-telemetry

# PVCs are retained by default — delete manually if needed
kubectl delete pvc -l app.kubernetes.io/instance=elastic-gpu-telemetry
```

### values.yaml overrides reference

| Key | Default | Description |
|-----|---------|-------------|
| `imagePullPolicy` | `IfNotPresent` | Applied to all containers |
| `questdb.storageSize` | `10Gi` | QuestDB data PVC size |
| `mq.replicas` | `1` | Keep at 1 (WAL is node-local) |
| `mq.walStorageSize` | `1Gi` | MQ WAL PVC size |
| `collector.replicas` | `2` | Number of Collector pods |
| `collector.config.workers` | `4` | ILP writer goroutines per Collector |
| `streamer.replicas` | `1` | Number of Streamer pods |
| `apiGateway.service.type` | `ClusterIP` | `ClusterIP`, `NodePort`, or `LoadBalancer` |
| `apiGateway.service.nodePort` | `30081` | NodePort value (when type=NodePort) |
| `ingress.enabled` | `false` | Enable Ingress for the API Gateway |

---

## API Reference

### `GET /api/v1/gpus`

Returns all GPU IDs that have telemetry in QuestDB.

```json
[{"id":"0"},{"id":"1"},{"id":"2"}]
```

### `GET /api/v1/gpus/{id}/telemetry`

Returns paginated telemetry entries for a single GPU.

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | `1` | Page number (1-indexed) |
| `page_size` | int | `100` | Rows per page (max `GATEWAY_PAGE_SIZE_MAX`) |
| `start_time` | RFC3339 | — | Start of time window (requires `end_time`) |
| `end_time` | RFC3339 | — | End of time window (requires `start_time`) |

**Response:**

```json
{
  "gpu_id": "0",
  "page": 1,
  "size": 3,
  "entries": [
    {
      "timestamp": "2026-05-06T13:16:02.406613Z",
      "metric_name": "DCGM_FI_DEV_MEM_CLOCK",
      "value": 2619,
      "hostname": "mtv5-dgx1-hgpu-028",
      "labels_raw": "..."
    }
  ]
}
```

**Status codes:** `200 OK`, `400 Bad Request` (invalid params), `404 Not Found` (unknown GPU), `500 Internal Server Error`.

### `GET /healthz`

Returns `{"status":"ok"}` with HTTP 200. Used by Kubernetes readiness and liveness probes.

---

## Configuration Reference

All configuration is via environment variables. Every component falls back to safe defaults if a variable is unset.

### MQ (`cmd/mq`)

| Variable | Default | Description |
|----------|---------|-------------|
| `MQ_LISTEN_ADDR` | `:7000` | TCP listen address |
| `MQ_WAL_DIR` | `./wal` | Directory for per-streamer WAL files |
| `MQ_SNAPSHOT_INTERVAL` | `30s` | How often to write the WAL checkpoint |
| `MQ_INFLIGHT_TIMEOUT` | `5s` | Time before an un-ACKed message is redelivered |
| `MQ_MAX_RETRIES` | `3` | Max redelivery attempts before dropping |
| `MQ_BUFFER_CAP` | `1000` | Per-streamer in-memory buffer capacity |

### Streamer (`cmd/streamer`)

| Variable | Default | Description |
|----------|---------|-------------|
| `STREAMER_MQ_ADDR` | `localhost:7000` | MQ TCP address |
| `STREAMER_ID` | hostname | Unique streamer identifier |
| `STREAMER_CSV_PATH` | _(required)_ | Path to the DCGM metrics CSV file |
| `STREAMER_SEND_RATE` | `0` | Delay between publishes (`0` = unlimited) |

### Collector (`cmd/collector`)

| Variable | Default | Description |
|----------|---------|-------------|
| `COLLECTOR_MQ_ADDR` | `localhost:7000` | MQ TCP address |
| `COLLECTOR_ID` | hostname | Unique collector identifier |
| `COLLECTOR_QUESTDB_ILP_ADDR` | `localhost:9009` | QuestDB ILP address |
| `COLLECTOR_QUESTDB_PG_ADDR` | `localhost:8812` | QuestDB pgwire address |
| `COLLECTOR_WORKERS` | `4` | Number of parallel ILP writer goroutines |

### API Gateway (`cmd/api-gateway`)

| Variable | Default | Description |
|----------|---------|-------------|
| `GATEWAY_LISTEN_ADDR` | `:8081` | HTTP listen address |
| `GATEWAY_QUESTDB_PG_ADDR` | `localhost:8812` | QuestDB pgwire address |
| `GATEWAY_PAGE_SIZE_MAX` | `1000` | Maximum page_size accepted by the API |

---

## Running Tests

```bash
# All unit tests with per-package coverage
make test

# Target: ≥70% per package (internal/*, cmd/api-gateway)
go tool cover -func=coverage.out | grep -E "total|70"

# Load test (10 streamers × 10 collectors × 1000 msgs, p99 < 50ms)
make test-load

# Single package
go test -v ./internal/mq/...
go test -v ./internal/protocol/...
go test -v ./cmd/api-gateway/...
```

Coverage summary from last run:

| Package | Coverage |
|---------|----------|
| `cmd/api-gateway` | 75.0% |
| `internal/config` | 90.5% |
| `internal/mq` | 84.6% |
| `internal/protocol` | 73.4% |
| `internal/telemetry` | 81.5% |
| `internal/questdb` | 14.5%* |

\* Most `internal/questdb` functions (`CreateTableIfNotExists`, `NewILPWriter`, `ListGPUs`) require a live QuestDB connection and are covered by integration testing only. The DDL content and `scanPage` logic are unit-tested via `export_test.go`.
