# Implementation Plan: Elastic GPU Telemetry Pipeline

**Branch**: `001-gpu-telemetry-pipeline` | **Date**: 2026-05-06 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-elastic-gpu-telemetry-pipeline/spec.md`

## Summary

Build an elastic, scalable, stable GPU telemetry pipeline in Go with four independently deployable components: a custom TCP Message Queue (MQ), a Telemetry Streamer (CSV→MQ), a Telemetry Collector (MQ→QuestDB), and a REST API Gateway (QuestDB→HTTP). The MQ implements a custom binary framing protocol over TCP with ACK/NACK at-least-once delivery, sticky-hash routing per streamer (guarantees per-streamer ordering), and hybrid in-memory/snapshot persistence.

## Technical Context

**Language/Version**: Go 1.22+  
**Primary Dependencies**: Standard library (`net`, `encoding/binary`, `sync`, `context`, `bufio`); `pgx/v5` for QuestDB queries (PostgreSQL wire); `questdb/go-questdb-client` or direct ILP for Collector writes; `go-chi/chi/v5` for HTTP routing; `swaggo/swag` for OpenAPI generation; `testify` for test assertions  
**Storage**: QuestDB single-node (ILP port 9009 for writes, PostgreSQL wire port 8812 for queries)  
**Testing**: `go test`, `testify/assert`, `net/http/httptest`  
**Target Platform**: Linux containers (Docker); Kubernetes / KIND for local cluster testing  
**Project Type**: Multi-service backend system — 4 independently deployable Go binaries  
**Performance Goals**: MQ hop p99 < 50ms; sustain 10 concurrent streamers × 10 concurrent collectors without degradation  
**Constraints**: No third-party MQ libraries (FR-001); custom binary TCP frames with 4-byte length prefix; at-least-once delivery via explicit ACK/NACK; in-memory buffer with periodic disk snapshots for MQ  
**Scale/Scope**: Up to 10 streamer + 10 collector instances; single CSV file per streamer instance

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

> **Note**: The project constitution (`.specify/memory/constitution.md`) has not been customized from its template. The gates below are derived directly from explicit spec requirements.

| Gate | Status | Notes |
|------|--------|-------|
| All components independently deployable | PASS | SC-005; each binary + Dockerfile is standalone |
| No third-party MQ libraries | PASS | FR-001 explicit; standard library only for TCP/binary |
| Go implementation | PASS | Spec Assumptions section: "Implementation will be in Golang" |
| ACK/NACK at-least-once delivery | REQUIRED | FR-005; in-flight tracker with timeout redelivery |
| Unit tests + measurable coverage | REQUIRED | SC-006; threshold set at 70% per component |
| Dockerfiles for all components | REQUIRED | SC-007; one Dockerfile per cmd/ binary |
| OpenAPI auto-generated via Makefile | REQUIRED | US-5 acceptance criterion; `make swagger` target |
| Per-streamer ordering preserved | REQUIRED | FR-009; sticky-hash routing by streamer ID |

**Post-Phase-1 Re-check**: All gates remain PASS/satisfiable after design phase. No violations requiring justification.

## Project Structure

### Documentation (this feature)

```text
specs/001-elastic-gpu-telemetry-pipeline/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   ├── api-v1.yaml      # OpenAPI 3.0 spec for REST API Gateway
│   └── mq-protocol.md  # MQ binary wire protocol specification
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created here)
```

### Source Code (repository root)

```text
cmd/
├── mq/            # Custom Message Queue server binary
├── streamer/      # Telemetry Streamer binary (CSV → MQ)
├── collector/     # Telemetry Collector binary (MQ → QuestDB)
└── api-gateway/   # REST API Gateway binary (QuestDB → HTTP)

internal/
├── protocol/      # Binary frame encoding/decoding, message type constants
├── mq/            # MQ core: router, in-memory buffer, in-flight tracker, snapshot WAL
├── telemetry/     # TelemetryRecord struct, CSV parser
├── questdb/       # QuestDB ILP writer (Collector) and pgx query client (API Gateway)
└── config/        # Config structs and env-var loading per component

deploy/
├── docker/        # Dockerfiles: mq.Dockerfile, streamer.Dockerfile, etc.
├── compose/       # docker-compose.yml (local dev)
└── k8s/           # KIND manifests for cluster testing

data/
└── input_dcgm_metrics_20250718_134233.csv

Makefile           # build, test, swagger, docker targets
go.mod
go.sum
```

**Structure Decision**: Option 3 — multi-binary monorepo. Each `cmd/<service>/main.go` produces one binary. Shared logic lives in `internal/`. This satisfies SC-005 (independent deployability) while keeping a single go.mod for easy cross-package refactoring.

## Complexity Tracking

> No constitution violations. No entries required.
