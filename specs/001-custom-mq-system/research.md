# Research Report: Elastic GPU Telemetry Pipeline

## Binary Protocol Design (MQ)
- **Decision**: Length-prefixed binary framing over TCP.
- **Rationale**: Provides low overhead and easy parsing in Go. Supports handshakes for Streamers and Collectors.
- **Format**: `[4 bytes: Total Length][1 byte: Message Type][16 bytes: Message ID][16 bytes: Streamer ID][Payload]`

## Persistence Layer (QuestDB)
- **Decision**: Use QuestDB with the PostgreSQL wire protocol.
- **Rationale**: QuestDB is optimized for time-series data and supports SQL queries. Using the Postgres wire protocol allows the Collector and API Gateway to use standard Go libraries (`pgx` or `database/sql`).
- **Single Node**: As per constitution, QuestDB remains a single node.

## API Gateway & OpenAPI
- **Decision**: REST API in Go using `chi` or `echo`. Auto-generate OpenAPI spec via `swag` or similar tool.
- **Rationale**: Provides a standard way to document and test APIs. Integration with Makefile ensures documentation stays in sync with code.

## Horizontal Scaling
- **Decision**: Round-robin message distribution in MQ.
- **Rationale**: Simple and effective for balancing load across up to 10 collector instances.
- **Ordering**: Per-streamer ordering maintained via source-affinity or sequential processing per Streamer ID.

## High Availability
- **Decision**: MQ periodic state snapshots to disk.
- **Rationale**: Allows MQ to recover pending messages after a restart.
