# Tasks: Custom Messaging Queue System

**Input**: Design documents from `/specs/001-custom-mq-system/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/protocol.md

**Tests**: Mandatory unit tests with measurable code coverage via Makefile (per constitution).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create project structure (mq/, streamer/, collector/, deploy/) per implementation plan
- [ ] T002 Initialize Go modules in mq/, streamer/, and collector/ directories
- [ ] T003 [P] Create root Makefile for project orchestration and building all modules
- [ ] T004 [P] Configure Dockerfiles for mq, streamer, and collector components

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core protocol and communication infrastructure

- [ ] T005 [P] Implement binary protocol header and message types in mq/pkg/protocol/protocol.go
- [ ] T006 [P] Implement binary protocol serialization/deserialization logic in mq/pkg/protocol/codec.go
- [ ] T007 [P] Create unit tests for protocol codec in mq/pkg/protocol/codec_test.go
- [ ] T008 Implement base TCP server in mq/pkg/server/server.go
- [ ] T009 [P] Implement basic in-memory message buffer in mq/pkg/queue/buffer.go
- [ ] T010 [P] Setup logging middleware in mq/pkg/server/middleware.go

**Checkpoint**: Foundation ready - binary protocol and basic server structure are in place.

---

## Phase 3: User Story 1 - Basic Telemetry Streaming (Priority: P1) 🎯 MVP

**Goal**: Enable Telemetry Streamer to connect to MQ and send telemetry data.

**Independent Test**: Run MQ server and Streamer client; verify Streamer can connect and MQ logs receipt of Handshake and Data messages.

### Tests for User Story 1

- [ ] T011 [P] [US1] Unit test for Streamer connection logic in streamer/pkg/client/client_test.go
- [ ] T012 [US1] Integration test for MQ receipt of streamer data in mq/pkg/server/integration_test.go

### Implementation for User Story 1

- [ ] T013 [P] [US1] Implement Streamer handshake logic (Type 0x01) in streamer/pkg/client/client.go
- [ ] T014 [US1] Implement CSV reading logic in streamer/pkg/reader/reader.go
- [ ] T015 [US1] Implement telemetry streaming loop in streamer/cmd/main.go
- [ ] T016 [US1] Implement MQ handler for Streamer handshake in mq/pkg/server/handler.go
- [ ] T017 [US1] Implement MQ receipt and ACK of Data messages (Type 0x03) in mq/pkg/server/handler.go

**Checkpoint**: User Story 1 complete - data can flow from Streamer to MQ.

---

## Phase 4: User Story 2 - Basic Telemetry Collection (Priority: P1)

**Goal**: Enable Telemetry Collector to connect to MQ and receive telemetry data.

**Independent Test**: Run MQ, Streamer, and Collector; verify Collector receives data sent by Streamer and sends ACKs back.

### Tests for User Story 2

- [ ] T018 [P] [US2] Unit test for Collector connection logic in collector/pkg/server/server_test.go
- [ ] T019 [US2] Integration test for end-to-end data flow (Streamer -> MQ -> Collector) in tests/e2e/streaming_test.go

### Implementation for User Story 2

- [ ] T020 [P] [US2] Implement Collector handshake logic (Type 0x02) in collector/pkg/server/server.go
- [ ] T021 [US2] Implement MQ handler for Collector handshake in mq/pkg/server/handler.go
- [ ] T022 [US2] Implement message dispatching logic in mq/pkg/queue/dispatcher.go
- [ ] T023 [US2] Implement Collector message receipt and ACK (Type 0x04) in collector/pkg/server/server.go
- [ ] T024 [US2] Implement simple persistence (file/log) in collector/pkg/store/store.go

**Checkpoint**: User Story 2 complete - full pipeline (Streamer -> MQ -> Collector) is functional.

---

## Phase 5: User Story 3 - Horizontal Scaling & Load Balancing (Priority: P2)

**Goal**: Distribute messages across multiple active collectors using Round-robin.

**Independent Test**: Run 1 Streamer and 2 Collectors; verify messages alternate between the two collectors.

### Tests for User Story 3

- [ ] T025 [US3] Load balancing integration test in mq/pkg/queue/balancer_test.go

### Implementation for User Story 3

- [ ] T026 [US3] Implement Round-robin distribution strategy in mq/pkg/queue/balancer.go
- [ ] T027 [US3] Implement per-streamer ordering logic in mq/pkg/queue/buffer.go
- [ ] T028 [US3] Implement unacknowledged message tracking and timeout re-queuing in mq/pkg/queue/buffer.go

**Checkpoint**: User Story 3 complete - system supports multiple collectors with fair distribution.

---

## Phase 6: User Story 4 - High Availability & Resilience (Priority: P3)

**Goal**: Ensure system stability under high load (10 instances) and implement periodic snapshots.

**Independent Test**: Scale to 10 instances in KIND and run load test; crash MQ and verify recovery from snapshot.

### Tests for User Story 4

- [ ] T029 [US4] Snapshot/Recovery unit test in mq/pkg/queue/snapshot_test.go
- [ ] T030 [US4] Load test script in tests/load/load_test.sh

### Implementation for User Story 4

- [ ] T031 [US4] Implement periodic state snapshotting to disk in mq/pkg/queue/snapshot.go
- [ ] T032 [US4] Implement MQ recovery from snapshot on startup in mq/cmd/main.go
- [ ] T033 [US4] Implement back-pressure (Reject/Block) logic when buffer is full in mq/pkg/server/server.go
- [ ] T034 [P] Create Helm charts for MQ, Streamer, and Collector in deploy/helm/

**Checkpoint**: User Story 4 complete - system is resilient and production-ready for the specified scale.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final cleanup, documentation, and validation

- [ ] T035 [P] Finalize root Makefile with `test`, `build`, and `deploy` targets
- [ ] T036 [P] Update README.md with architecture overview and usage instructions
- [ ] T037 Perform code refactoring for performance and idiomatic Go
- [ ] T038 Validate quickstart.md against the final implementation
- [ ] T039 [P] Ensure all components have Dockerfiles and are KIND-compatible

---

## Phase 8: Create Documentation
**Purpose**: All features, design, steps to run the modules etc. should be documented.

- [ ] T040 An overall README.md file at the top explaining features in brief and the commands to execute the full set of modules and see the output.
- [ ] T041 An individual README.md for each module explaining the features, design and all the relevant commands to execute and test the module.

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Can start immediately.
- **Foundational (Phase 2)**: Depends on Phase 1 completion.
- **User Story 1 (Phase 3)**: Depends on Phase 2 completion.
- **User Story 2 (Phase 4)**: Depends on US1 (Phase 3) completion.
- **User Story 3 (Phase 5)**: Depends on US2 (Phase 4) completion.
- **User Story 4 (Phase 6)**: Depends on US3 (Phase 5) completion.
- **Polish (Phase 7)**: Depends on all user stories being complete.

### Parallel Opportunities

- Dockerfiles (T004) and Makefile (T003) can be created in parallel.
- Protocol logic (T005, T006) and tests (T007) can be developed in parallel with server/queue foundations (T008, T009).
- Handshake logic for Streamer (T013) and Collector (T020) can be implemented in parallel.
- Documentation and final Makefile targets (T035, T036) can be done in parallel during Polish phase.

---

## Parallel Example: Phase 2 Foundations

```bash
# Implement protocol and server components in parallel:
Task: "Implement binary protocol header and message types in mq/pkg/protocol/protocol.go"
Task: "Implement base TCP server in mq/pkg/server/server.go"
Task: "Implement basic in-memory message buffer in mq/pkg/queue/buffer.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 & 2)

1. Complete Setup and Foundational phases.
2. Complete US1 (Streaming) and US2 (Collection).
3. **STOP and VALIDATE**: Verify end-to-end telemetry flow through the custom MQ.

### Incremental Delivery

1. Add Scaling & Load Balancing (US3) once the basic pipeline is stable.
2. Add Resilience and HA (US4) last to harden the system for scale.
3. Polish and document once all functionality is verified.
