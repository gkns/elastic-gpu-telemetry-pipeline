# Feature Specification: Custom Messaging Queue System

**Feature Branch**: `001-custom-mq-system`  
**Created**: 2026-04-30  
**Status**: Draft  
**Input**: User description: "Design and implement an elastic, scalable, and stable telemetry pipeline for an AI Cluster with a custom messaging queue. The system will process GPU telemetry data from CSV files and expose it via a REST API. The core modules which are independently deployable are : - Custom Messaging Queue, which does not use any existing open-source message queues. This MQ connects the streamers and collectors. Scalable upto 10 instances for streamer/collector. ..."

## Clarifications

### Session 2026-04-30
- Q: Should the MQ persist messages to disk? → A: Hybrid (Memory prioritized, periodic snapshots).
- Q: Should the MQ implement authentication? → A: None (Assumes trusted private network).
- Q: Is strict global ordering required? → A: Per-streamer (Order maintained for individual source).
- Q: Which transport protocol to use? → A: TCP (Custom Binary Framing).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Custom Message Queue (Priority: P0)

As a system operator, I want the custom messaging queue to be independently deployable and scalable so that I can scale it up or down based on the telemetry load.

**Why this priority**: Core functionality of the pipeline; without message transmission, no telemetry can be processed.

**Independent Test**: Can be tested by running one MQ instance and one Streamer sending a known set of metrics, verifying the MQ accepts the connection and receives the data.

**Acceptance Scenarios**:

1. **Given** the MQ is running, **When** a Streamer attempts to connect, **Then** the connection is established successfully.
2. **Given** a connected Streamer, **When** it sends a telemetry message, **Then** the MQ acknowledges receipt.

---

### User Story 2 - Basic Telemetry Streaming (Priority: P1)

As a Telemetry Streamer, I want to connect to the custom messaging queue and send telemetry data so that it can be processed by collectors.

**Why this priority**: Core functionality of the pipeline; without message transmission, no telemetry can be processed.

**Independent Test**: Can be tested by running one MQ instance and one Streamer sending a known set of metrics, verifying the MQ accepts the connection and receives the data.

**Acceptance Scenarios**:

1. **Given** the MQ is running, **When** a Streamer attempts to connect, **Then** the connection is established successfully.
2. **Given** a connected Streamer, **When** it sends a telemetry message, **Then** the MQ acknowledges receipt.

---

### User Story 2 - Basic Telemetry Collection (Priority: P1)

As a Telemetry Collector, I want to connect to the custom messaging queue and receive telemetry data so that I can parse and persist it.

**Why this priority**: Essential for data processing; collectors are needed to make the data useful.

**Independent Test**: Can be tested by running one MQ instance with pre-loaded or currently arriving messages and verifying the Collector receives them in the correct order.

**Acceptance Scenarios**:

1. **Given** the MQ has pending messages, **When** a Collector connects, **Then** it receives the messages.
2. **Given** a connected Collector, **When** it receives a message, **Then** it sends an acknowledgment back to the MQ.

---

### User Story 3 - Horizontal Scaling & Load Balancing (Priority: P2)

As a system operator, I want the custom messaging queue to distribute messages among multiple collectors so that the system can handle high telemetry volumes.

**Why this priority**: Key requirement for "elastic and scalable" pipeline.

**Independent Test**: Can be tested by running one MQ, one Streamer sending 100 messages, and two Collectors. Verify that both Collectors receive a portion of the messages (e.g., roughly 50 each).

**Acceptance Scenarios**:

1. **Given** multiple connected Collectors, **When** a Streamer sends multiple messages, **Then** the messages are distributed across all active Collectors.
2. **Given** a Collector instance is added or removed, **When** messages are flowing, **Then** the MQ rebalances the distribution without losing data.

---

### User Story 4 - High Availability & Resilience (Priority: P3)

As a system operator, I want the MQ to handle up to 10 instances of streamers and collectors without crashing or losing data.

**Why this priority**: Ensures the "stable" aspect of the pipeline under maximum specified load.

**Independent Test**: Can be tested by simulating 10 Streamers and 10 Collectors in a load test environment and verifying system stability and message integrity over a sustained period.

**Acceptance Scenarios**:

1. **Given** 10 Streamer and 10 Collector instances, **When** high-frequency telemetry is streamed, **Then** the MQ maintains stable resource usage and zero message loss.

---

### Edge Cases

- **What happens when a Collector crashes?**: The MQ should detect the disconnection and re-route unacknowledged messages to other active collectors.
- **How does the system handle MQ saturation?**: If all collectors are busy and the MQ buffer is full, the MQ MUST block or reject new incoming messages (Back-pressure) to preserve system stability.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST implement a custom binary communication protocol over TCP (not using third-party MQ libraries).
- **FR-002**: System MUST support concurrent connections from multiple Telemetry Streamers (producers).
- **FR-003**: System MUST support concurrent connections from multiple Telemetry Collectors (consumers).
- **FR-004**: System MUST implement a load distribution strategy (e.g., Round-robin) to balance messages across active collectors.
- **FR-005**: System MUST implement an acknowledgment mechanism (ACK/NACK) to ensure at-least-once delivery.
- **FR-006**: System MUST support scaling up to 10 instances each for streamers and collectors.
- **FR-007**: System MUST provide a mechanism for service discovery or connection coordination (e.g., MQ address/port).
- **FR-008**: System MUST assume a trusted network environment (no built-in authentication required for this phase).
- **FR-009**: System MUST guarantee per-streamer message ordering (messages from a single source arrive at the collector in the order they were sent).

### Key Entities

- **Message**: Represents the atomic unit of telemetry data being transferred, containing metadata (timestamp, ID) and payload (CSV line content).
- **Message Queue**: The Custom Message Queue is a distributed messaging system that enables communication between Telemetry Streamers and Telemetry Collectors.
- **Producer Node**: A Telemetry Streamer instance that maintains a connection to the MQ.
- **Consumer Node**: A Telemetry Collector instance that subscribes to messages from the MQ.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Message delivery latency (MQ hop) remains under 50ms for 99% of messages under standard load.
- **SC-002**: System successfully handles 10 concurrent streamers and 10 concurrent collectors without performance degradation.
- **SC-003**: Zero message loss confirmed during graceful scaling (adding/removing collectors).
- **SC-004**: System recovers and re-distributes messages within 5 seconds of a collector failure.
- **SC-004**: All components are independently deployable and scalable.
- **SC-006**: All components have unit tests with measurable code coverage.
- **SC-007**: All components have corresponding dockerfiles.

## Assumptions

- **Language Choice**: Implementation will be in Golang as per the project constitution.
- **Deployment**: Components will be containerized and deployed on Kubernetes (KIND for local testing).
- **Data Persistence**: MQ prioritizes in-memory storage for performance but performs periodic state snapshots to disk for crash recovery; full persistence is handled by the Collector.
- **Network Stack**: Custom binary protocol with length-prefixed framing over TCP.
binary protocol with length-prefixed framing over TCP.
