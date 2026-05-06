# Research: Elastic GPU Telemetry Pipeline

**Phase**: 0 — Research  
**Date**: 2026-05-06  
**Resolves**: All NEEDS CLARIFICATION items from Technical Context

---

## 1. Go TCP Binary Framing Pattern

**Decision**: Goroutine-per-connection with `bufio.Reader`, 4-byte big-endian length-prefixed frames.

**Rationale**: For ≤20 simultaneous connections (10 streamers + 10 collectors) goroutine-per-connection is idiomatic and has no measurable overhead. `bufio.Reader` amortizes syscall cost via buffering. `encoding/binary.BigEndian` handles the length prefix. A `sync.Pool` for byte buffers prevents GC pressure on hot paths.

**Frame layout**:
```
┌─────────────┬──────────────┬──────────────┬────────────────────┐
│  Type (1B)  │  Length (4B) │  MsgID (8B)  │  Body (Length B)   │
└─────────────┴──────────────┴──────────────┴────────────────────┘
Total header: 13 bytes
```

**Alternatives considered**:
- Worker-pool (fixed goroutines): rejected — unnecessary complexity at this connection count.
- Protocol Buffers framing: rejected — adds external dependency; `encoding/binary` is sufficient and zero-dep.
- TLV (type-length-value) without fixed header size: rejected — variable header complicates parsing.

---

## 2. QuestDB Integration Strategy

**Decision**: Collector writes via **ILP (InfluxDB Line Protocol)** on port 9009; API Gateway queries via **`pgx/v5`** on PostgreSQL wire port 8812.

**Rationale**:
- ILP is QuestDB's recommended high-throughput write path — it bypasses SQL parsing and goes directly to the storage engine. The official `questdb/go-questdb-client` library handles ILP framing and connection management.
- `pgx/v5` is the de-facto Go PostgreSQL driver. QuestDB's PostgreSQL compatibility layer (port 8812) supports `SELECT` queries, aggregations, and time-range filters needed by the API Gateway.
- Separating the write path (ILP) from the read path (pgx) avoids contention and uses each protocol at its strength.

**QuestDB table**:
```sql
CREATE TABLE IF NOT EXISTS gpu_telemetry (
  ts            TIMESTAMP,
  metric_name   SYMBOL,
  gpu_id        SYMBOL,
  device        SYMBOL,
  uuid          SYMBOL,
  model_name    SYMBOL,
  hostname      SYMBOL,
  container     SYMBOL,
  pod           SYMBOL,
  namespace     SYMBOL,
  value         DOUBLE,
  labels_raw    STRING,
  streamer_id   SYMBOL
) TIMESTAMP(ts) PARTITION BY DAY WAL;
```

`SYMBOL` columns are dictionary-encoded — ideal for the low-cardinality metric/GPU/host strings. `PARTITION BY DAY` keeps queries against time ranges efficient. `WAL` mode ensures crash-safe writes.

**Alternatives considered**:
- HTTP REST API (port 9000): supports both reads and writes but higher per-request overhead than ILP; rejected for write path.
- Using `pgx` for writes: works but slower than ILP for bulk time-series inserts; rejected.

---

## 3. Per-Streamer Ordering with Multi-Collector Distribution (FR-004 + FR-009)

**Decision**: **Sticky routing by streamer ID** using consistent hash (FNV-1a mod N collectors).

**Rationale**: Pure round-robin at the message level would send consecutive messages from the same streamer to different collectors, violating FR-009 (per-streamer ordering). Sticky routing maps each streamer to exactly one collector at any moment, preserving order while still distributing load across collectors proportionally to the number of active streamers.

When a collector disconnects, the MQ rehashes affected streamer→collector mappings to remaining live collectors and replays un-ACKed in-flight messages for those streamers to the new target.

**Implementation**:
```go
func collectorForStreamer(streamerID string, collectors []string) string {
    h := fnv.New32a()
    h.Write([]byte(streamerID))
    return collectors[int(h.Sum32())%len(collectors)]
}
```

**Alternatives considered**:
- Round-robin per message: violated FR-009; rejected.
- Consistent hash ring (Rendezvous hashing): more complex, marginal benefit at ≤10 collectors; rejected.
- Dedicated queue per streamer with separate consumer: more resource-heavy; would require per-streamer goroutine pools; rejected for initial implementation.

---

## 4. ACK/NACK Protocol and At-Least-Once Delivery (FR-005)

**Decision**: Two-phase ACK: Streamer→MQ ACK (receipt) and Collector→MQ ACK (processed). In-flight tracker with 5-second timeout and 3-retry limit before redelivery to next collector.

**Flow**:
```
Streamer ──PUBLISH──► MQ ──ACK──► Streamer  (receipt confirmed)
MQ ──DELIVER──► Collector ──ACK──► MQ       (processing confirmed)
     [5s timeout]
MQ ──DELIVER──► NextCollector                (if no ACK received)
```

**In-flight tracker**:
```go
type InFlight struct {
    MsgID       uint64
    CollectorID string
    SentAt      time.Time
    Retries     int
    Payload     []byte
}
```

A `time.Ticker` scans the in-flight map every second and redelivers expired entries.

**Alternatives considered**:
- Single ACK (streamer only): no delivery guarantee to collector; rejected.
- Persistent Kafka-style offset commit: requires disk coordination; overkill for this scale; rejected.

---

## 5. In-Memory Buffer with Disk Snapshots (Spec Assumption)

**Decision**: Per-streamer FIFO ring buffer in memory + periodic WAL append to disk, snapshot every 30 seconds.

**Structure**:
- Each streamer connection gets a `chan []byte` (buffered channel, capacity 1000 messages) as its queue.
- A background goroutine writes incoming messages to a WAL file (`wal/<streamer_id>.wal`) in append-only mode.
- Every 30 seconds, a snapshot goroutine writes a checkpoint file with the current in-flight state.
- On startup, the MQ replays from the WAL since the last checkpoint.

**Back-pressure**: If a streamer's channel is full (buffer saturated), the MQ blocks the `PUBLISH` read from that TCP connection (FR-004 edge case: "block or reject new incoming messages").

**Alternatives considered**:
- Global single queue: simpler but loses per-streamer ordering and creates a single point of saturation; rejected.
- mmap-based ring buffer: higher complexity, not needed at this scale; rejected.
- No persistence (pure memory): violates the spec assumption; rejected.

---

## 6. OpenAPI Generation (US-5 Acceptance Criterion)

**Decision**: `swaggo/swag` with `swag init` run from `cmd/api-gateway/`. Makefile target: `make swagger`.

**Rationale**: `swaggo/swag` is the standard Go OpenAPI generator — annotations in handler code produce `docs/swagger.json` and `docs/swagger.yaml`. The `swaggerui` embed can serve the UI at `/swagger/` if desired.

**Makefile target**:
```makefile
swagger:
	swag init -g cmd/api-gateway/main.go -o specs/001-elastic-gpu-telemetry-pipeline/contracts/
```

---

## 7. HTTP Router: chi vs gin

**Decision**: `go-chi/chi/v5`.

**Rationale**: `chi` uses standard `net/http.Handler` interfaces — no framework lock-in, easy to test with `httptest.NewRecorder`. `swaggo/swag` integrates well with chi. At this traffic volume the performance difference with gin is immaterial.

**Alternatives considered**:
- `gin`: faster, but opinionated context type complicates standard middleware composition; rejected.
- `net/http` only: viable but adds boilerplate for path parameter extraction; rejected.

---

## 8. Local Development: Docker Compose vs KIND

**Decision**: Docker Compose for local development, KIND manifests for cluster integration tests.

**Rationale**: Docker Compose is faster to iterate on (single `docker compose up`). KIND is needed to validate Kubernetes-specific behavior (service discovery, horizontal scaling). Both are required by SC-005.

**docker-compose.yml** will include: `questdb`, `mq`, `streamer` (1 instance), `collector` (1 instance), `api-gateway`.

KIND manifests will include Deployments, Services, and ConfigMaps for each component with replica counts.

---

## Resolved Clarifications Summary

| Unknown | Resolution |
|---------|-----------|
| TCP framing approach | 4-byte length-prefix + 1-byte type + 8-byte MsgID header; goroutine-per-conn |
| QuestDB write protocol | ILP via `questdb/go-questdb-client` (Collector) |
| QuestDB query protocol | `pgx/v5` PostgreSQL wire (API Gateway) |
| Multi-collector routing | Sticky FNV-1a hash by streamer ID mod N collectors |
| Per-streamer ordering mechanism | Sticky routing; rehash + replay on collector disconnect |
| ACK/NACK timeout | 5-second in-flight timeout, 3 retries, then reroute |
| MQ buffer | Per-streamer buffered channel (cap 1000) + WAL + 30s checkpoint |
| OpenAPI tooling | `swaggo/swag`, `make swagger` target |
| HTTP router | `go-chi/chi/v5` |
| Local dev setup | Docker Compose (dev) + KIND (cluster tests) |
