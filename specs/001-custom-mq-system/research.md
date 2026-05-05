# Research Report: Custom Messaging Queue System

## Binary Protocol Design
- **Decision**: Length-prefixed binary framing over TCP.
- **Rationale**: Provides low overhead and easy parsing in Go.
- **Format**: 
  - Header: `[4 bytes: Total Length][1 byte: Message Type][16 bytes: Message ID][16 bytes: Streamer ID]`
  - Message Types: `0x01: Handshake (Streamer)`, `0x02: Handshake (Collector)`, `0x03: Telemetry Data`, `0x04: ACK`, `0x05: NACK`.
- **Alternatives**: JSON-over-TCP (too verbose), WebSockets (added HTTP overhead).

## Reliability & Delivery
- **Decision**: In-memory message tracking with timeout-based NACK.
- **Rationale**: MQ tracks unacknowledged messages. If a collector fails or times out, the message is re-queued for the next available collector (Round-Robin).
- **At-least-once**: Achieved via mandatory ACKs from collectors.

## Per-streamer Ordering
- **Decision**: Sequential processing per Streamer ID.
- **Rationale**: By including `Streamer ID` in the header, the MQ can ensure that messages from the same ID are processed in FIFO order even if multiple collectors are active (e.g., by pinning a Streamer ID to a specific queue segment or ensuring sequential dispatch).
- **Implementation**: MQ will maintain a map of active Streamer IDs and their last sequence to ensure order before load balancing to collectors.

## Persistence & Snapshots
- **Decision**: Copy-on-write snapshots to disk.
- **Rationale**: To avoid blocking the hot path, the MQ will periodically trigger a background routine to serialize the current in-memory queue state to a `.bin` file.

## K8s Service Discovery
- **Decision**: Kubernetes ClusterIP Service.
- **Rationale**: The MQ will be exposed via a stable Service name (e.g., `mq-service`) that Streamers and Collectors can use as their connection endpoint.
