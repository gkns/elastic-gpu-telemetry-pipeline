# Implementation Plan: Custom Messaging Queue System

**Branch**: `001-custom-mq-system` | **Date**: 2026-04-30 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-custom-mq-system/spec.md`

## Summary

Design and implement a custom, high-performance messaging queue using a binary protocol over TCP in Golang. The system will facilitate reliable, at-least-once telemetry delivery from streamers to collectors, supporting horizontal scaling and per-streamer ordering.

## Technical Context

**Language/Version**: Golang 1.21+ (Clean, idiomatic)  
**Primary Dependencies**: Go Standard Library (`net`, `encoding/binary`, `sync`), Docker, Kubernetes (KIND), Helm  
**Storage**: In-memory (hot path), Disk (periodic snapshots for crash recovery)  
**Testing**: Go `testing` package, `Makefile` for coverage  
**Target Platform**: Linux/Docker/Kubernetes (KIND)
**Project Type**: Distributed Messaging Service  
**Performance Goals**: < 50ms message latency (MQ hop), Support for 10+ concurrent streamer/collector instances  
**Constraints**: Custom binary protocol ONLY (no third-party MQ libraries), At-least-once delivery (ACK/NACK), Round-robin load distribution  
**Scale/Scope**: Telemetry pipeline for AI Clusters, processing high-volume GPU metrics from CSV inputs.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **No Third-Party MQ**: Custom implementation as required.
- [x] **Independent Modules**: Separate directories and Dockerfiles for MQ, Streamer, and Collector.
- [x] **Inter-module Communication**: All communication will flow through the custom MQ protocol.
- [x] **Tech Stack**: Golang, Docker, K8s (KIND), Helm, Makefile.
- [x] **Testing**: Unit tests and coverage included in plan.

## Project Structure

### Documentation (this feature)

```text
specs/001-custom-mq-system/
├── spec.md              # Feature Specification
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── checklists/          # Quality validation checklists
```

### Source Code (repository root)

```text
mq/                      # Custom Messaging Queue implementation
├── cmd/                 # Entry point
├── pkg/
│   ├── protocol/        # Binary protocol logic
│   ├── server/          # TCP server and connection handling
│   └── queue/           # In-memory storage and load balancing
├── Dockerfile
└── Makefile

streamer/                # Telemetry Streamer implementation
├── cmd/
├── pkg/
│   ├── reader/          # CSV parsing
│   └── client/          # MQ protocol client
├── Dockerfile
└── Makefile

collector/               # Telemetry Collector implementation
├── cmd/
├── pkg/
│   ├── server/          # MQ protocol client (consumer)
│   └── store/           # Persistence logic
├── Dockerfile
└── Makefile

deploy/                  # Deployment configuration
├── helm/                # Helm charts (MQ, Streamer, Collector)
└── kind/                # KIND cluster setup scripts

Makefile                 # Root Makefile for orchestration
```

**Structure Decision**: Monorepo with dedicated root directories for each component to ensure independent deployability and clear separation of concerns as mandated by the constitution.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

*No constitution violations identified. Implementation follows all mandated principles.*
