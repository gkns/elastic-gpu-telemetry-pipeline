# Tasks: Elastic GPU Telemetry Pipeline

**Input**: Design documents from `/specs/001-custom-mq-system/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/api.md

**Tests**: Mandatory unit tests with measurable code coverage via Makefile (per constitution).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4, US5, US6, US7)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create project structure (mq/, streamer/, collector/, api-gateway/, deploy/) per implementation plan
- [ ] T002 Initialize Go modules in each module directory
- [ ] T003 [P] Create root Makefile for project orchestration and building all modules
- [ ] T004 [P] Configure Dockerfiles for all components

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core protocol and communication infrastructure

- [ ] T005 [P] Implement binary protocol codec in mq/pkg/protocol/codec.go
- [ ] T010 [P] Implement base TCP server in mq/pkg/server/server.go
- [ ] T011 [P] Implement basic in-memory message buffer in mq/pkg/queue/buffer.go
- [ ] T012 Setup logging middleware in mq/pkg/server/middleware.go

**Checkpoint**: Foundation ready - binary protocol and basic server structure are in place.

---

## Phase 3: User Story 1 - Custom Messaging Queue (Priority: P0) 🎯 MVP

**Goal**: Establish the MQ server capable of handling handshakes and data.

**Independent Test**: Verify MQ server accepts TCP connections and correctly parses a "Hello" handshake.

- [ ] T013 [US1] Implement MQ handler for Streamer handshake (0x01) in mq/pkg/server/handler.go
- [ ] T014 [US1] Implement MQ handler for Collector handshake (0x02) in mq/pkg/server/handler.go
- [ ] T015 [US1] Implement MQ receipt and ACK logic for Data messages (0x03) in mq/pkg/server/handler.go

---

## Phase 4: User Story 2 - Single node QuestDB Setup (Priority: P0)

**Goal**: Provision and configure QuestDB for telemetry persistence.

**Independent Test**: Verify connectivity to QuestDB via Postgres wire protocol and create telemetry table.

- [ ] T016 [US2] Create Docker Compose or Helm chart for QuestDB in deploy/questdb/
- [ ] T017 [US2] Implement telemetry table migration script in deploy/migrations/001_create_telemetries.sql
- [ ] T018 [US2] Implement QuestDB connection pooling logic in collector/pkg/store/db.go

---

## Phase 5: User Story 3 - Basic Telemetry Streaming (Priority: P1)

**Goal**: Enable Streamer to read CSV and push to MQ with system processing time.

**Independent Test**: Run Streamer and MQ; verify MQ receives data rows with injected timestamps.

- [ ] T019 [US3] Implement CSV reading logic in streamer/pkg/reader/csv.go
- [ ] T020 [US3] Implement timestamp injection logic in streamer/pkg/processor/time.go
- [ ] T021 [US3] Implement MQ client and streaming loop in streamer/cmd/main.go

---

## Phase 6: User Story 4 - Basic Telemetry Collection & Persistence (Priority: P1)

**Goal**: Enable Collector to consume from MQ and persist to QuestDB.

**Independent Test**: Verify data flows from Streamer -> MQ -> Collector -> QuestDB.

- [ ] T022 [US4] Implement MQ consumer logic in collector/pkg/client/mq_client.go
- [ ] T023 [US4] Implement QuestDB ingestion logic (Postgres protocol) in collector/pkg/store/telemetry.go
- [ ] T024 [US4] Implement message ACK logic after successful DB persistence in collector/cmd/main.go

---

## Phase 7: User Story 5 - API Gateway (Priority: P1)

**Goal**: Serve telemetry data via REST API with auto-generated OpenAPI spec.

**Independent Test**: Verify GET /api/v1/gpus returns list of GPUs from QuestDB.

- [ ] T025 [US5] Implement QuestDB query logic for GPUs and telemetry in api-gateway/pkg/store/queries.go
- [ ] T026 [US5] Implement REST handlers for GPU and Telemetry endpoints in api-gateway/pkg/api/handlers.go
- [ ] T027 [US5] Integrate OpenAPI/Swagger generation in api-gateway/main.go and root Makefile
- [ ] T028 [US5] Implement pagination logic for telemetry queries in api-gateway/pkg/api/middleware.go

---

## Phase 8: User Story 6 - Horizontal Scaling & Load Balancing (Priority: P2)

**Goal**: Distribute messages across multiple collectors using Round-robin and per-streamer ordering.

**Independent Test**: Scale collectors to 2 and verify messages from a single streamer arrive in order.

- [ ] T029 [US6] Implement Round-robin distribution in mq/pkg/queue/balancer.go
- [ ] T030 [US6] Implement per-streamer sequence tracking in mq/pkg/queue/buffer.go
- [ ] T031 [US6] Implement unacknowledged message timeout and re-queue logic in mq/pkg/queue/dispatcher.go

---

## Phase 9: User Story 7 - High Availability & Resilience (Priority: P3)

**Goal**: Implement MQ snapshots and back-pressure.

**Independent Test**: Crash MQ and verify pending messages are recovered on restart.

- [ ] T032 [US7] Implement MQ state snapshotting to disk in mq/pkg/queue/snapshot.go
- [ ] T033 [US7] Implement MQ startup recovery logic in mq/cmd/main.go
- [ ] T034 [US7] Implement TCP back-pressure (blocking write/read) in mq/pkg/server/conn.go

---

## Phase 10: Polish & Documentation

**Purpose**: Final cleanup and cross-component validation.

- [ ] T035 [P] Implement performance benchmarks for MQ hop latency (SC-001) in tests/benchmarks/
- [ ] T036 [P] Ensure unit test coverage >= 80% for all modules (SC-006)
- [ ] T037 [P] Finalize root README.md with full pipeline execution guide
- [ ] T038 Create Helm charts for all pipeline components in deploy/helm/
- [ ] T039 [P] Validate quickstart.md against the end-to-end implementation

---

## Dependencies & Execution Order

1. **Setup (Phases 1-2)**: Prerequisite for all other work.
2. **Infrastructure (Phases 3-4)**: Setup MQ and DB.
3. **Core Pipeline (Phases 5-6)**: Streamer and Collector implementation.
4. **Access Layer (Phase 7)**: API Gateway depends on data being persisted.
5. **Scale & Stability (Phases 8-9)**: Enhancements to the core pipeline.
6. **Polish (Phase 10)**: Final validation and documentation.

## Parallel Example: User Story 5 API Gateway

```bash
# Parallel tasks for API Gateway:
Task: "Implement QuestDB query logic for GPUs and telemetry in api-gateway/pkg/store/queries.go"
Task: "Implement REST handlers for GPU and Telemetry endpoints in api-gateway/pkg/api/handlers.go"
```
