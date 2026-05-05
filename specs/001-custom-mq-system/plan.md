# Implementation Plan: Elastic GPU Telemetry Pipeline

**Branch**: `001-gpu-telemetry-pipeline` | **Date**: 2026-04-30 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-custom-mq-system/spec.md`

## Summary

Design and implement a complete telemetry pipeline for an AI Cluster. The system includes a custom high-performance messaging queue (binary protocol over TCP), telemetry streamers reading from CSV, telemetry collectors persisting to QuestDB, and an API Gateway for querying data via REST. All components are containerized and deployed via Helm on Kubernetes.

## Technical Context

**Language/Version**: Golang 1.21+ (Clean, idiomatic)  
**Primary Dependencies**: Go Standard Library (`net`, `encoding/binary`, `sync`), QuestDB (PostgreSQL wire protocol for ingestion/querying), Docker, Kubernetes (KIND), Helm, OpenAPI/Swagger  
**Storage**: QuestDB (Single-node time-series persistence), In-memory (MQ hot path), Disk (MQ periodic snapshots)  
**Testing**: Go `testing` package, `Makefile` for coverage and benchmarks  
**Target Platform**: Linux/Docker/Kubernetes (KIND)
**Project Type**: Distributed Systems / Telemetry Pipeline  
**Performance Goals**: < 50ms message latency (MQ hop), Support for 10+ concurrent streamer/collector instances  
**Constraints**: Custom binary protocol ONLY (no third-party MQ libraries), Independently deployable modules (except QuestDB), Makefile-driven build and doc generation  
**Scale/Scope**: Telemetry pipeline for AI Clusters, processing high-volume GPU metrics from CSV inputs.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Custom MQ**: Custom binary protocol implementation as required.
- [x] **QuestDB Persistence**: Using QuestDB for time-series data storage.
- [x] **Independent Modules**: Separate directories/Dockerfiles for MQ, Streamer, Collector, and API Gateway.
- [x] **Inter-module Communication**: Streamer/Collector talk through custom MQ. Collector/Gateway talk through QuestDB.
- [x] **Tech Stack**: Golang, Docker, K8s, Helm, Makefile.
- [x] **Testing**: Unit tests and coverage mandated via Makefile.

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
mq/                      # Custom Messaging Queue (TCP/Binary)
streamer/                # CSV Telemetry Streamer
collector/               # MQ Consumer and QuestDB Persister
api-gateway/             # REST API for Telemetry Querying
deploy/                  # Helm charts and KIND setup
Makefile                 # Orchestration, builds, and doc generation
```

**Structure Decision**: Monorepo with dedicated root directories for each component to ensure independent deployability and clear separation of concerns as mandated by the constitution.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

*No violations identified. Architecture follows all constitution mandates including the API Gateway requirement.*
