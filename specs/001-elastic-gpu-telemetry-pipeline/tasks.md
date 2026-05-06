# Tasks: Elastic GPU Telemetry Pipeline

**Input**: Design documents from `/specs/001-elastic-gpu-telemetry-pipeline/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓, quickstart.md ✓

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.
**Tests**: Included — SC-006 explicitly requires unit tests with ≥70% coverage per package.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no shared write dependencies)
- **[Story]**: Which user story this task belongs to ([US1]–[US7])
- Exact file paths are included in all descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the monorepo scaffold, initialize the Go module, add all third-party dependencies, and write the Makefile. Nothing else can build until this phase is complete.

- [ ] T001 Create full directory scaffold at repository root: `cmd/mq/`, `cmd/streamer/`, `cmd/collector/`, `cmd/api-gateway/`, `internal/protocol/`, `internal/mq/`, `internal/telemetry/`, `internal/questdb/`, `internal/config/`, `deploy/docker/`, `deploy/compose/`, `deploy/k8s/`
- [ ] T002 Initialize `go.mod` at repository root declaring module `github.com/gkns/elastic-gpu-telemetry-pipeline` and Go 1.22; run `go get` to add all direct dependencies: `github.com/jackc/pgx/v5`, `github.com/questdb/go-questdb-client/v3`, `github.com/go-chi/chi/v5`, `github.com/swaggo/swag`, `github.com/swaggo/http-swagger`, `github.com/stretchr/testify`; commit resulting `go.mod` and `go.sum`
- [ ] T003 [P] Create `Makefile` at repository root with targets: `build` (compiles all four binaries to `bin/mq`, `bin/streamer`, `bin/collector`, `bin/api-gateway`), `test` (runs `go test -coverprofile=coverage.out ./...` and prints per-package coverage), `test-load` (runs `go test -tags loadtest -timeout 120s -run TestLoad ./internal/mq/` to validate SC-001/SC-002), `swagger` (placeholder — see T031 for full `swag init` invocation), `docker-build` (builds `deploy/docker/*.Dockerfile` images), `lint` (runs `golangci-lint run ./...`), `clean` (removes `bin/` and `coverage.*`)
- [ ] T004 [P] Create `.gitignore` additions (if not already present) to exclude `bin/`, `coverage.*`, `wal/`, `docs/` (swag output when generated locally)

**Checkpoint**: `go build ./...` should succeed (with empty main stubs); `make build` should produce four binaries in `bin/`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Implement the shared packages that every component depends on — configuration loading and the binary wire-protocol codec. No user story can start until both packages compile and are tested.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T005 Implement `internal/config/config.go` — define `MQConfig`, `StreamerConfig`, `CollectorConfig`, `APIGatewayConfig` structs exactly as specified in data-model.md §7; implement `LoadMQConfig()`, `LoadStreamerConfig()`, `LoadCollectorConfig()`, `LoadAPIGatewayConfig()` functions that read each field from the corresponding environment variable (e.g., `MQ_LISTEN_ADDR`) with the documented defaults when the variable is unset
- [ ] T006 [P] Implement `internal/protocol/frame.go` — define `MsgType` (uint8) constants `MsgTypePublish=0x01` through `MsgTypeHeartbeat=0x06`; define `NodeType` constants `NodeTypeStreamer=0x01`, `NodeTypeCollector=0x02`; define `Frame` struct `{Type MsgType; MsgID uint64; Body []byte}`; define `HelloBody`, `PublishBody` structs as specified in data-model.md §1 with serialisation/deserialisation methods using `encoding/binary` big-endian
- [ ] T007 [P] Implement `internal/protocol/codec.go` — implement `ReadFrame(r *bufio.Reader) (*Frame, error)` that reads the 13-byte header (1B Type + 4B Length + 8B MsgID) then reads `Length` bytes of body; implement `WriteFrame(w io.Writer, f *Frame) error` that writes the header then the body; use `encoding/binary.BigEndian`; add a `sync.Pool` for the read buffer to avoid per-frame heap allocation; write unit tests in `internal/protocol/codec_test.go`

**Checkpoint**: `go test ./internal/config/... ./internal/protocol/...` passes; both packages compile cleanly.

---

## Phase 3: User Story 1 — Custom Message Queue (Priority: P0) 🎯 MVP Core

**Goal**: A standalone MQ binary that listens on TCP :7000, performs HELLO handshakes, routes PUBLISH frames from Streamers to Collectors using sticky FNV-1a hashing, sends ACK/NACK, tracks in-flight messages, appends to a WAL, and emits heartbeats. This is the backbone all other components depend on.

**Independent Test**: `./bin/mq` starts and listens; a raw TCP client sends a HELLO frame (NodeType=Streamer, ID="test-1") and receives ACK(MsgID=0).

### Implementation for User Story 1

- [ ] T008 [P] [US1] Implement `internal/mq/queue.go` — define `StreamerQueue` struct `{StreamerID string; Ch chan *InFlightMessage; LastSeq uint64}` as in data-model.md §3; implement `NewStreamerQueue(id string, cap int) *StreamerQueue`; implement `Enqueue(msg *InFlightMessage) error` that validates SeqNum is strictly greater than LastSeq and sends to Ch (non-blocking, returns error if full)
- [ ] T009 [P] [US1] Implement `internal/mq/inflight.go` — define `InFlightMessage` struct as in data-model.md §3; implement `InFlightTracker` with a `sync.Mutex`-protected `map[uint64]*InFlightMessage`; implement `Track(msg)`, `Ack(msgID uint64)`, `Expired(timeout time.Duration) []*InFlightMessage` (returns all entries older than timeout with Retries < 3); increment Retries before returning from Expired
- [ ] T010 [US1] Implement `internal/mq/router.go` — define `CollectorConn` struct `{ID string; Conn net.Conn; Send chan *Frame}`; implement `Router` with a `sync.RWMutex`-protected slice of active `CollectorConn`; implement `Register(c *CollectorConn)`, `Deregister(id string) []*InFlightMessage` (returns all in-flight messages assigned to that collector for redelivery), `RouteForStreamer(streamerID string) *CollectorConn` using FNV-1a hash `fnv.New32a()` mod len(active collectors) as in research.md §3; implement `CollectorIDs() []string`
- [ ] T011 [US1] Implement `internal/mq/wal.go` — implement `WAL` struct backed by an append-only file at `<walDir>/<streamerID>.wal`; implement `Append(entry WALEntry) error` writing the binary layout from data-model.md §3 (SeqNum 8B, StreamerIDLen 2B, StreamerID N, PayloadLen 4B, Payload N); implement `WriteCheckpoint(cp Checkpoint) error` writing JSON to `<walDir>/checkpoint.json`; implement `ReplayFrom(cp Checkpoint) ([]*InFlightMessage, error)` that reads WAL entries since the checkpoint offset for each streamer
- [ ] T012 [US1] Implement `cmd/mq/main.go` — load `MQConfig` from env; start TCP listener on `MQConfig.ListenAddr`; spawn `handleConn(conn net.Conn, router *Router, tracker *InFlightTracker, wal *WAL)` goroutine per connection that: reads HELLO frame (asserts it is the first frame), registers the node as Streamer or Collector; for Streamers: enters a read loop processing PUBLISH frames — validates SeqNum via StreamerQueue, ACKs the Streamer, appends to WAL, enqueues to StreamerQueue; for Collectors: starts a write goroutine draining its `Send` channel; start a dispatcher goroutine dequeuing from all StreamerQueues and routing to CollectorConn.Send via router; start a redelivery ticker (every 1s) scanning expired in-flight messages and redelivering via router; start a WAL snapshot ticker (every `SnapshotInterval`) writing checkpoint; start a heartbeat sender per connection (every 10s); handle graceful shutdown with context cancellation
- [ ] T013 [P] [US1] Write unit tests `internal/mq/queue_test.go`, `internal/mq/inflight_test.go`, `internal/mq/router_test.go` — test SeqNum validation (in-order pass, out-of-order reject), InFlightTracker Ack removes entry and Expired returns only timed-out entries, Router sticky routing returns same collector for same streamer ID, rehash after Deregister routes to remaining collectors
- [ ] T014 [US1] Create `deploy/docker/mq.Dockerfile` — two-stage build: stage 1 `FROM golang:1.22-alpine AS builder` copies go.mod/go.sum, runs `go mod download`, copies source, runs `go build -o /app/mq ./cmd/mq`; stage 2 `FROM alpine:3.19` copies binary, exposes port 7000, sets `ENTRYPOINT ["/app/mq"]`
- [ ] T049 set conn.SetReadDeadline(time.Now().Add(30s)) after each successful read in cmd/mq/main.go; on timeout, trigger router.Deregister and
    clean up goroutines.

**Checkpoint**: `./bin/mq` starts without error; `go test ./internal/mq/...` passes with ≥70% coverage.

---

## Phase 4: User Story 2 — QuestDB Persistence Layer (Priority: P0)

**Goal**: QuestDB running as a Docker service; Go ILP writer and pgx query client implemented; `gpu_telemetry` table DDL created at Collector startup. Enables both Collector writes (US4) and API Gateway queries (US5).

**Independent Test**: Start QuestDB via `docker run questdb/questdb:latest -p 8812:8812 -p 9009:9009`; run unit tests for `internal/questdb/` with a real QuestDB connection; verify `SELECT DISTINCT gpu_id FROM gpu_telemetry` returns an empty result set (table exists).

### Implementation for User Story 2

- [ ] T015 [P] [US2] Implement `internal/questdb/schema.go` — implement `CreateTableIfNotExists(ctx context.Context, pgAddr string) error` that opens a `pgx/v5` connection to `pgAddr` and executes the `CREATE TABLE IF NOT EXISTS gpu_telemetry (...)` DDL from data-model.md §4 (columns: ts TIMESTAMP, metric_name SYMBOL, gpu_id SYMBOL, device SYMBOL, uuid SYMBOL, model_name SYMBOL, hostname SYMBOL, container SYMBOL, pod SYMBOL, namespace SYMBOL, value DOUBLE, labels_raw STRING, streamer_id SYMBOL) TIMESTAMP(ts) PARTITION BY DAY WAL
- [ ] T016 [P] [US2] Implement `internal/questdb/ilp_writer.go` — define `ILPWriter` wrapping `questdb/go-questdb-client` sender; implement `NewILPWriter(addr string) (*ILPWriter, error)`; implement `Write(ctx context.Context, rec *telemetry.TelemetryRecord) error` that sends one ILP line to table `gpu_telemetry` with all columns from TelemetryRecord (use `rec.ProcessedAt` as the designated timestamp); implement `Flush(ctx context.Context) error` and `Close() error`
- [ ] T017 [US2] Implement `internal/questdb/pg_client.go` — define `PGClient` wrapping `pgx/v5` pool; implement `NewPGClient(ctx context.Context, pgAddr string) (*PGClient, error)`; implement `ListGPUs(ctx context.Context) ([]string, error)` executing `SELECT DISTINCT gpu_id FROM gpu_telemetry`; implement `GetTelemetry(ctx context.Context, gpuID string, page, pageSize int) (*TelemetryPage, error)` executing the paginated query from data-model.md §4; implement `GetTelemetryRange(ctx context.Context, gpuID string, start, end time.Time, page, pageSize int) (*TelemetryPage, error)` executing the time-range query; define `TelemetryRow` struct for scan targets
- [ ] T018 [P] [US2] Write unit tests `internal/questdb/schema_test.go` and `internal/questdb/pg_client_test.go` — use `pgxmock` or an interface abstraction to mock the pgx pool; test `ListGPUs` returns the correct slice from mock rows, `GetTelemetry` constructs correct SQL with correct LIMIT/OFFSET for page 2 page_size 10, `GetTelemetryRange` adds correct WHERE ts BETWEEN clause
- [ ] T019 [US2] Create `deploy/compose/docker-compose.yml` — define services: `questdb` (image `questdb/questdb:latest`, ports `8812:8812 9009:9009 9000:9000`, volume `questdb-data:/root/.questdb`), `mq` (build `deploy/docker/mq.Dockerfile`, env `MQ_LISTEN_ADDR=:7000`, depends_on mq starts first), `streamer` (build `deploy/docker/streamer.Dockerfile`, env `STREAMER_MQ_ADDR=mq:7000 STREAMER_CSV_PATH=/data/input_dcgm_metrics_20250718_134233.csv`, volume `./data:/data`, depends_on mq), `collector` (build `deploy/docker/collector.Dockerfile`, env `COLLECTOR_MQ_ADDR=mq:7000 COLLECTOR_QUESTDB_ILP_ADDR=questdb:9009 COLLECTOR_QUESTDB_PG_ADDR=questdb:8812`, depends_on mq and questdb), `api-gateway` (build `deploy/docker/api-gateway.Dockerfile`, env `GATEWAY_QUESTDB_PG_ADDR=questdb:8812`, ports `8080:8080`, depends_on questdb); add named volume `questdb-data`

**Checkpoint**: `docker compose -f deploy/compose/docker-compose.yml up questdb` starts cleanly; `go test ./internal/questdb/...` passes.

---

## Phase 5: User Story 3 — Basic Telemetry Streaming (Priority: P1)

**Goal**: Streamer binary reads `data/input_dcgm_metrics_20250718_134233.csv` row-by-row, injects `time.Now()` as `ProcessedAt`, JSON-encodes each row as `TelemetryRecord`, wraps it in a PUBLISH frame with monotonic SeqNum, sends to MQ via TCP, and handles ACK (advance) / NACK (retry with backoff).

**Independent Test**: Start MQ (`./bin/mq`); run `STREAMER_CSV_PATH=data/input_dcgm_metrics_20250718_134233.csv ./bin/streamer`; observe MQ logs showing PUBLISH frames received and ACKed.

### Implementation for User Story 3

- [ ] T020 [P] [US3] Implement `internal/telemetry/record.go` — define `TelemetryRecord` struct exactly as in data-model.md §2 with all fields and JSON tags: `ProcessedAt time.Time`, `MetricName`, `GPUID`, `Device`, `UUID`, `ModelName`, `Hostname`, `Container`, `Pod`, `Namespace` (string), `Value float64`, `LabelsRaw string`, `StreamerID string`
- [ ] T021 [US3] Implement `internal/telemetry/csv_parser.go` — implement `NewCSVParser(path string) (*CSVParser, error)` that opens the file and reads the header row; implement `Next() (*TelemetryRecord, error)` that reads the next CSV row, maps columns to TelemetryRecord fields per data-model.md §2 column mapping table (CSV `timestamp` column is IGNORED; set `ProcessedAt = time.Now().UTC()`; parse `value` as `strconv.ParseFloat`); return `io.EOF` when done; implement `Close() error`
- [ ] T022 [US3] Implement `cmd/streamer/main.go` — load `StreamerConfig` from env; dial TCP to `MQAddr`; send HELLO frame (`NodeType=NodeTypeStreamer`, `ID=StreamerConfig.StreamerID`); read ACK(MsgID=0) or fail; open `CSVParser` for `CSVPath`; start heartbeat goroutine sending `MsgTypeHeartbeat` every 10s; enter publish loop: read next record from parser, JSON-marshal to `TelemetryRecord`, build `PublishBody` (StreamerID, SeqNum++, Timestamp=record.ProcessedAt.UnixNano(), Payload=JSON bytes), wrap in PUBLISH Frame with new MsgID, call `WriteFrame`; await response: on ACK advance SeqNum; on NACK (any ReasonCode) wait 100ms and retry same frame without incrementing SeqNum; if `SendRate > 0` sleep between sends; log each PUBLISH/ACK pair with row index to stderr
- [ ] T023 [P] [US3] Write unit tests `internal/telemetry/csv_parser_test.go` — test `Next()` on the first 3 rows of `data/input_dcgm_metrics_20250718_134233.csv`: verify `ProcessedAt` is non-zero and after a reference time, `MetricName = "DCGM_FI_DEV_GPU_UTIL"`, `GPUID = "0"`, `Value = 0.0`, CSV `timestamp` column is ignored (not copied to any field)
- [ ] T024 [US3] Create `deploy/docker/streamer.Dockerfile` — two-stage build: stage 1 FROM golang:1.22-alpine copies go.mod/go.sum, runs `go mod download`, copies source, runs `go build -o /app/streamer ./cmd/streamer`; stage 2 FROM alpine:3.19 copies binary and `data/` directory, sets `ENTRYPOINT ["/app/streamer"]`

**Checkpoint**: `./bin/streamer` with a running MQ sends all CSV rows and exits cleanly; `go test ./internal/telemetry/...` passes.

---

## Phase 6: User Story 4 — Basic Telemetry Collection (Priority: P1)

**Goal**: Collector binary connects to MQ as a consumer (NodeType=Collector), receives DELIVER frames, deserialises the embedded PublishBody to extract JSON TelemetryRecord, writes to QuestDB via ILP, and sends ACK back to MQ. Runs WorkerCount parallel ILP writer goroutines.

**Independent Test**: Start MQ and Streamer; run `./bin/collector`; verify rows appear in QuestDB via `SELECT count() FROM gpu_telemetry` (should be > 0).

### Implementation for User Story 4

- [ ] T025 [US4] Implement `cmd/collector/main.go` — load `CollectorConfig` from env; call `questdb.CreateTableIfNotExists(ctx, CollectorConfig.QuestDBPGAddr)` at startup; create `ILPWriter` pool of `WorkerCount` goroutines each owning an `ILPWriter` instance, fed via a `chan *telemetry.TelemetryRecord`; dial TCP to `MQAddr`; send HELLO (`NodeType=NodeTypeCollector`, `ID=CollectorConfig.CollectorID`); read ACK(MsgID=0); start heartbeat goroutine; enter DELIVER receive loop: read Frame, assert `MsgTypeDeliver`, deserialise body as `PublishBody` (StreamerID, SeqNum, Ts, Payload), JSON-unmarshal Payload into `TelemetryRecord`, set `record.ProcessedAt = time.Unix(0, publishBody.Ts).UTC()`, send record to ILP worker channel; ILP worker calls `writer.Write(ctx, record)` then `writer.Flush(ctx)` — on write failure, retry up to 3 times with 100ms/500ms/1s backoff; if all 3 retries fail, send a NACK frame back to the MQ (MsgType=MsgTypeNACK, AckMsgID=frame.MsgID) so the message is requeued for redelivery rather than silently dropped; on success send ACK frame back to MQ with AckMsgID = frame.MsgID; log throughput (rows/sec) to stderr every 5 seconds
- [ ] T026 [P] [US4] Write unit tests `cmd/collector/collector_test.go` — define a `TelemetryWriter` interface with `Write` and `Flush` methods; mock it; test that given a DELIVER frame with a known PublishBody, the collector: decodes TelemetryRecord correctly, sets ProcessedAt from Ts field (not CSV timestamp), calls `writer.Write` once, sends ACK with correct AckMsgID
- [ ] T027 [US4] Create `deploy/docker/collector.Dockerfile` — two-stage build: stage 1 FROM golang:1.22-alpine, `go build -o /app/collector ./cmd/collector`; stage 2 FROM alpine:3.19, copies binary, sets `ENTRYPOINT ["/app/collector"]`

**Checkpoint**: Full pipeline (MQ + Streamer + Collector + QuestDB) runs end-to-end; rows appear in QuestDB; `go test ./cmd/collector/...` passes.

---

## Phase 7: User Story 5 — API Gateway (Priority: P1)

**Goal**: REST API Gateway serving `GET /api/v1/gpus`, `GET /api/v1/gpus/{id}/telemetry` (with optional time-range and pagination), and `GET /healthz` via chi router, backed by pgx QuestDB queries. OpenAPI spec auto-generated by `make swagger`.

**Independent Test**: Insert mock rows into QuestDB; run `./bin/api-gateway`; `curl http://localhost:8080/api/v1/gpus` returns a JSON array with at least one GPU ID.

### Implementation for User Story 5

- [ ] T028 [P] [US5] Implement `cmd/api-gateway/models.go` — define `GPU struct {ID string \`json:"id"\`}`, `TelemetryEntry` struct (Timestamp time.Time, MetricName string, Value float64, Hostname string, LabelsRaw string) with JSON tags, `TelemetryPage` struct (GPUID, Page, Size, Total int, Entries []TelemetryEntry) with JSON tags; add swaggo model annotations (`// swagger:model GPU` etc.)
- [ ] T029 [US5] Implement `cmd/api-gateway/handlers.go` — create `Handlers` struct holding `*questdb.PGClient`; implement `ListGPUs(w, r)` calling `pgClient.ListGPUs`, marshalling to `[]GPU`; implement `GetGPUTelemetry(w, r)` parsing path param `id` (chi.URLParam), query params `start_time`, `end_time` (ISO 8601, optional), `page` (default 1), `page_size` (default 100, max 1000), calling `pgClient.GetTelemetry` or `GetTelemetryRange` depending on whether time params are present, marshalling to `TelemetryPage`; implement `HealthCheck(w, r)` returning `{"status":"ok"}`; add swaggo annotations on each handler: `// @Summary`, `// @Tags`, `// @Param`, `// @Success 200`, `// @Failure 400/404/500`, `// @Router`; add `writeError(w, code int, msg string)` helper
- [ ] T030 [US5] Implement `cmd/api-gateway/main.go` — load `APIGatewayConfig` from env; create `PGClient`; create chi router with `chi.NewRouter()`; mount middleware `middleware.Logger`, `middleware.Recoverer`; register routes: `GET /api/v1/gpus → handlers.ListGPUs`, `GET /api/v1/gpus/{id}/telemetry → handlers.GetGPUTelemetry`, `GET /healthz → handlers.HealthCheck`; start `http.ListenAndServe(ListenAddr, router)`
- [ ] T031 [US5] Fill in the `swagger` Makefile target created in T003 with the full invocation: `swag init -g cmd/api-gateway/main.go -o specs/001-elastic-gpu-telemetry-pipeline/contracts/`; run `make swagger` to verify it produces `contracts/swagger.json` and `contracts/swagger.yaml` without errors
- [ ] T032 [P] [US5] Write unit tests `cmd/api-gateway/handlers_test.go` — define `GPUQuerier` interface with `ListGPUs`, `GetTelemetry`, `GetTelemetryRange` methods; implement a mock; test `ListGPUs` returns 200 JSON array; test `GetGPUTelemetry` with no time params returns 200 TelemetryPage; test `GetGPUTelemetry` with both `start_time` and `end_time` calls `GetTelemetryRange`; test `GetGPUTelemetry` with invalid `page_size=0` returns 400; test unknown GPU ID returns 404
- [ ] T033 [US5] Create `deploy/docker/api-gateway.Dockerfile` — two-stage build: stage 1 FROM golang:1.22-alpine, `go build -o /app/api-gateway ./cmd/api-gateway`; stage 2 FROM alpine:3.19, copies binary, exposes port 8080, sets `ENTRYPOINT ["/app/api-gateway"]`

**Checkpoint**: `curl http://localhost:8080/api/v1/gpus` returns valid JSON; `make swagger` regenerates `contracts/swagger.json` without errors; `go test ./cmd/api-gateway/...` passes.

---

## Phase 8: User Story 6 — Horizontal Scaling & Load Balancing (Priority: P2)

**Goal**: MQ distributes messages across multiple Collectors using sticky FNV-1a hash routing. Adding or removing a Collector causes un-ACKed in-flight messages to be rehashed and redelivered to remaining Collectors within the ACK timeout window, with zero message loss.

**Independent Test**: `docker compose up --scale collector=2`; run Streamer; verify both Collector instances log received messages and MQ logs show routing to both collector IDs.

### Implementation for User Story 6

- [ ] T034 [US6] Harden `internal/mq/router.go` for concurrent scaling scenarios: add integration test `internal/mq/router_scale_test.go` that creates a `Router`, registers 2 `CollectorConn` objects, sends 100 messages via `RouteForStreamer` for 10 streamer IDs, deregisters one Collector, verifies all 10 streamers rehash to the remaining Collector, and re-registers a new Collector to verify load rebalances; assert zero panics and deterministic routing
- [ ] T035 [P] [US6] Update `deploy/compose/docker-compose.yml` to add `deploy: replicas: 1` stubs (no-op for `docker compose up`) and add a comment showing the scale command `docker compose up --scale collector=3 --scale streamer=2`; create `deploy/k8s/kind-config.yaml` — KIND cluster config with 1 control-plane node
- [ ] T036 [US6] Create KIND Kubernetes manifests in `deploy/k8s/`: `questdb-deployment.yaml` (1 replica, ports 8812/9009, PVC), `mq-deployment.yaml` (1 replica, port 7000, ClusterIP Service `mq:7000`), `streamer-deployment.yaml` (1 replica, env STREAMER_MQ_ADDR=mq:7000, volume mount for CSV ConfigMap), `collector-deployment.yaml` (2 replicas, env COLLECTOR_MQ_ADDR=mq:7000), `api-gateway-deployment.yaml` (1 replica, NodePort Service port 8080), `configmap-csv.yaml` (embeds first 100 rows of CSV for smoke testing)
- [ ] T037 [P] [US6] Write `internal/mq/router_scale_test.go` — integration-style test (no network, pure in-memory): register 10 `CollectorConn` (with nil Conn, buffered Send channels); route 1000 messages for 10 streamer IDs; assert each streamer always maps to the same collector; deregister 5 collectors; assert all streamers still route without panic; assert no two streamers share a mapping conflict

**Checkpoint**: `docker compose up --scale collector=2` runs without error; KIND manifests apply cleanly (`kubectl apply -f deploy/k8s/ --dry-run=client`); `go test ./internal/mq/...` passes.

---

## Phase 9: User Story 7 — High Availability & Resilience (Priority: P3)

**Goal**: MQ survives restarts by replaying WAL from the last checkpoint; in-flight timeout redelivery works correctly after Collector disconnect; back-pressure blocks the Streamer TCP read (not NACK) when the per-streamer buffer is full; system handles 10×10 load without message loss.

**Independent Test**: Start MQ and Streamer; kill MQ mid-stream; restart MQ; observe Streamer reconnects and un-ACKed messages are replayed from WAL.

### Implementation for User Story 7

- [ ] T038 [US7] Complete `internal/mq/wal.go` WAL replay: implement `ReplayFrom(walDir string, cp Checkpoint) ([]*InFlightMessage, error)` that opens each `<walDir>/<streamerID>.wal` file, seeks to `cp.Streamers[id].WALOffset`, reads all subsequent WAL entries into `InFlightMessage` list with `Retries=0`, and returns them for redelivery; hook this into `cmd/mq/main.go` startup after loading checkpoint — enqueue replayed messages into the appropriate `StreamerQueue`
- [ ] T039 [US7] Implement in-flight redelivery ticker in `cmd/mq/main.go` — start a `time.NewTicker(1 * time.Second)` goroutine that calls `tracker.Expired(MQConfig.InFlightTimeout)`, iterates results, and for each entry: if `Retries < MQConfig.MaxRetries` calls `router.RouteForStreamer(entry.StreamerID)` and sends a new DELIVER frame via `CollectorConn.Send`, updates `entry.SentAt = time.Now()` and re-tracks; if `Retries >= MaxRetries` removes from tracker, logs "message dropped after max retries", increments a drop counter; write unit test `internal/mq/inflight_redelivery_test.go` verifying redelivery is attempted up to MaxRetries and then dropped
- [ ] T040 [P] [US7] Implement back-pressure in the MQ Streamer connection handler in `cmd/mq/main.go` — when `StreamerQueue.Enqueue()` returns an error (channel full), do NOT send NACK immediately; instead pause reading from that TCP connection by blocking on a `bufferReady` channel or by stopping the read goroutine until the queue drains below 50% capacity; when capacity is available, resume reading and ACK the frame; write a comment explaining why blocking the TCP read (not NACK) is the correct back-pressure signal
- [ ] T041 [US7] Write load-test `internal/mq/load_test.go` (tagged `//go:build loadtest`) — spin up 10 mock streamer goroutines and 10 mock collector goroutines communicating through the MQ Router and InFlightTracker in-process (no real TCP); stream 1000 messages per streamer (10,000 total); assert zero message loss (every message ACKed by a collector); assert p99 router dispatch latency < 50ms using `time.Since`; run with `go test -tags loadtest -run TestLoad ./internal/mq/`
- [ ] T048 Implement exponential-backoff reconnect loop in cmd/streamer/main.go and cmd/collector/main.go (max 5s between retries, log each
    attempt).



**Checkpoint**: Killing and restarting MQ during active Streamer resumes from WAL; `go test -tags loadtest ./internal/mq/` passes under the 50ms p99 target.

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Validate the full pipeline end-to-end, ensure Makefile targets all work, hit ≥70% test coverage, and confirm Docker images build.

- [ ] T042 [P] Run `go vet ./...` and `staticcheck ./...` and fix all reported issues across all packages
- [ ] T043 Run `make build` and verify all four binaries (`bin/mq`, `bin/streamer`, `bin/collector`, `bin/api-gateway`) are produced without errors
- [ ] T044 Run `make test` and confirm per-package coverage is ≥70% for `internal/protocol`, `internal/config`, `internal/mq`, `internal/telemetry`, `internal/questdb`, `cmd/api-gateway`; add missing unit tests to any package below threshold
- [ ] T045 [P] Run `make docker-build` to build all four Dockerfiles in `deploy/docker/`; verify each image starts and prints its default config to stderr without crashing
- [ ] T046 Run `make swagger` and verify `specs/001-elastic-gpu-telemetry-pipeline/contracts/swagger.json` is valid and contains paths `/api/v1/gpus` and `/api/v1/gpus/{id}/telemetry`
- [ ] T047 Execute quickstart.md Option A end-to-end: start QuestDB container, run `make build`, start `bin/mq`, `bin/collector`, `bin/api-gateway`, `bin/streamer`; verify `curl http://localhost:8080/api/v1/gpus` returns at least one GPU ID from the CSV file

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Requires Phase 1 complete — BLOCKS all user story phases
- **US1 (Phase 3) and US2 (Phase 4)**: Both require Foundational; can proceed in parallel
- **US3 (Phase 5)**: Requires US1 (MQ binary must accept connections)
- **US4 (Phase 6)**: Requires US1 (DELIVER/ACK) and US2 (ILP writer, DDL)
- **US5 (Phase 7)**: Requires US2 (pgx client)
- **US6 (Phase 8)**: Requires US1 (router) and US3+US4 for realistic integration
- **US7 (Phase 9)**: Requires US1 (WAL, in-flight) fully functional
- **Polish (Phase 10)**: Requires all desired stories complete

### User Story Dependencies

| Story | Depends On | Can Parallelize With |
|-------|-----------|---------------------|
| US1 (MQ) | Foundational | US2 |
| US2 (QuestDB) | Foundational | US1 |
| US3 (Streamer) | US1 | US5 |
| US4 (Collector) | US1, US2 | US5 |
| US5 (API Gateway) | US2 | US3, US4 |
| US6 (Scaling) | US1, US3, US4 | — |
| US7 (HA) | US1 fully complete | — |

### Within Each User Story

- Models/structs before services
- Services before main binaries
- Unit tests alongside implementation (same [P] group)
- Dockerfile last (binary must compile first)

### Parallel Opportunities

- T006, T007 (protocol codec + constants) can run in parallel
- T008, T009, T013 (queue, inflight, tests) can run in parallel within US1
- T015, T016, T018 (ILP writer, pgx client, tests) can run in parallel within US2
- T020, T023 (record struct + tests) can run in parallel within US3
- T028, T032 (models + handler tests) can run in parallel within US5
- T035, T037 (docker compose + router scale test) can run in parallel within US6
- T042, T045, T046 (vet, docker-build, swagger) can run in parallel within Polish

---

## Parallel Example: User Story 1 (MQ)

```bash
# Launch in parallel (different files, no shared write):
Task T008: "Implement internal/mq/queue.go"
Task T009: "Implement internal/mq/inflight.go"
Task T013: "Write unit tests internal/mq/*_test.go"

# Then sequentially (depends on T008 + T009):
Task T010: "Implement internal/mq/router.go"

# Then sequentially (depends on T010):
Task T011: "Implement internal/mq/wal.go"
Task T012: "Implement cmd/mq/main.go"
```

## Parallel Example: User Story 5 (API Gateway)

```bash
# Launch in parallel (different files):
Task T028: "Implement cmd/api-gateway/models.go"
Task T032: "Write unit tests cmd/api-gateway/handlers_test.go"

# Then sequentially (depends on T028):
Task T029: "Implement cmd/api-gateway/handlers.go"
Task T030: "Implement cmd/api-gateway/main.go"
Task T031: "Add swagger Makefile target and verify"
```

---

## Implementation Strategy

### MVP First (US1 + US2 = Core Data Path)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (config + protocol codec)
3. Complete Phase 3: US1 — MQ binary running
4. Complete Phase 4: US2 — QuestDB client + docker-compose
5. **STOP and VALIDATE**: `./bin/mq` accepts connections; `docker compose up questdb` starts cleanly
6. Deploy/demo the MQ in isolation

### Incremental Delivery

1. Setup + Foundational → scaffold ready
2. US1 + US2 in parallel → MQ + QuestDB clients ready
3. US3 (Streamer) → publish telemetry to MQ → validate with MQ ACK logs
4. US4 (Collector) → persist to QuestDB → validate with `SELECT count() FROM gpu_telemetry`
5. US5 (API Gateway) → serve telemetry via REST → validate with `curl /api/v1/gpus`
6. US6 (Scaling) → multi-collector → validate with `--scale collector=2`
7. US7 (HA) → WAL replay + load test → validate resilience
8. Polish → coverage, Docker images, end-to-end quickstart

### Parallel Team Strategy

With 2+ developers after Phase 2 is complete:
- Developer A: US1 (MQ core) → US6 (scaling) → US7 (HA)
- Developer B: US2 (QuestDB) → US4 (Collector) → US5 (API Gateway)
- Developer C (optional): US3 (Streamer, depends on A's US1)

---

## Notes

- [P] tasks modify different files and have no shared write dependencies within the same phase
- [Story] labels trace each task to its user story for independent validation
- Each user story has a **Checkpoint** describing the observable proof of completion
- The WAL and in-flight redelivery (US7) build on top of US1 structures — do not skip US1 tasks
- `swag init` requires the `swag` CLI; install with `go install github.com/swaggo/swag/cmd/swag@latest` before running `make swagger`
- Coverage threshold is 70% per package (not aggregate); `go test -coverprofile` reports per-package
