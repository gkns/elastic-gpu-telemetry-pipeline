# Feature Specification: Elastic GPU Telemetry Pipeline

**Feature Branch**: `001-gpu-telemetry-pipeline`  
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

### User Story 2 - Single node QuestDB for persistence (Priority: P0)

For the collector to persist the telemetry and the API gateway to query, we need a time-series persistence layer.

**Why this priority**: This is essential for the collector to persist the telemetry and the API-Gateway to query and render the data.

**Independent Test**: This shall be unit-testable by inserting mock data into itself and running a few time-range queries.

**Acceptance Scenarios**:

1. **Given** the QuestDB is running, the collector should be able to persist the processed telemetry to the QuestDB through standard Go postgres drivers.

---

### User Story 3 - Basic Telemetry Streaming (Priority: P1)

As a Telemetry Streamer, I want to connect to the custom messaging queue and send telemetry data so that it can be processed by collectors.

A sample line with title and data from the telemetry data CSV is given below:

```timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_UTIL","0","nvidia0","GPU-5fd4f087-86f3-7a43-b711-4771313afc50","NVIDIA H100 80GB HBM3","mtv5-dgx1-hgpu-031","","","","0","DCGM_FI_DRIVER_VERSION=""535.129.03"",Hostname=""mtv5-dgx1-hgpu-031"",UUID=""GPU-5fd4f087-86f3-7a43-b711-4771313afc50"",__name__=""DCGM_FI_DEV_GPU_UTIL"",device=""nvidia0"",gpu=""0"",instance=""mtv5-dgx1-hgpu-031:9400"",job=""dgx_dcgm_exporter"",modelName=""NVIDIA H100 80GB HBM3"""
```

**Why this priority**: Core functionality of the pipeline; without message transmission, no telemetry can be processed.

**Independent Test**: Can be tested by running one MQ instance and one Streamer sending a known set of metrics, verifying the MQ accepts the connection and receives the data.

**Acceptance Scenarios**:

1. **Given** the MQ is running, **When** a Streamer attempts to connect, **Then** the connection is established successfully.
2. **Given** a connected Streamer, **When** it sends a telemetry message, **Then** the MQ acknowledges receipt.
3. When the Streamer reads a row from the CSV, it MUST inject the current system processing time as the timestamp for that telemetry payload.

---

### User Story 4 - Basic Telemetry Collection (Priority: P1)

As a Telemetry Collector, I want to connect to the custom messaging queue and receive telemetry data so that I can parse and persist it. The collector shall persist the data to a single-node **QuestDB**.

**Why this priority**: Essential for data processing; collectors are needed to make the data useful and highly desireable for this to be in time-series DB for efficient querying.

**Independent Test**: Can be tested by running one MQ instance with pre-loaded or currently arriving messages and verifying the Collector receives them in the correct order.

**Acceptance Scenarios**:

1. **Given** the MQ has pending messages, **When** a Collector connects, **Then** it receives the messages.
2. **Given** a connected Collector, **When** it receives a message, **Then** it sends an acknowledgment back to the MQ.

---

### User Story 5 - API Gateway (Priority: P1)

As an API gateway, I want to be able to serve the telemetry data to the user / other tools so that users can get the data or other tools can render the data. The API gateway queries the data in the QuestDB and renders the data.

**Why this priority**: Users should be able to make use of the data.

**Independent Test**: Can be tested by running one API gateway instance and some mocked data inserted into the QuestDB.

**Acceptance Scenarios**:

1. **Given** the collector persisted the data, the API gateway should be able to successfully query the database and return the results.
2. The OpenAPI (Swagger) specification for these endpoints MUST be auto-generated, triggered via a specific Makefile command.
2. **Given** mocked data in unit tests, the gateway should should be able to serve the following APIs at the minimum:

- [ ] `GET /api/v1/gpus` which returns a list of all GPUs for which telemetry data is available.
- [ ] `GET / api/v1/gpus/{id}/telemetry` which return a paginated telemetry entries for a specific GPU, ordered by time
- [ ] `GET / api/v1/gpus/{id}/telemetry?start_time=...&end_time=...` which returns a paginated telemetry entries for a specific GPU, within the specific time range and ordered by time.

---


### User Story 6 - Horizontal Scaling & Load Balancing (Priority: P2)x	

As a system operator, I want the custom messaging queue to distribute messages among multiple collectors so that the system can handle high telemetry volumes.

**Why this priority**: Key requirement for "elastic and scalable" pipeline.

**Independent Test**: Can be tested by running one MQ, one Streamer sending 100 messages, and two Collectors. Verify that both Collectors receive a portion of the messages (e.g., roughly 50 each).

**Acceptance Scenarios**:

1. **Given** multiple connected Collectors, **When** a Streamer sends multiple messages, **Then** the messages are distributed across all active Collectors.
2. **Given** a Collector instance is added or removed, **When** messages are flowing, **Then** the MQ rebalances the distribution without losing data.

---

### User Story 7 - High Availability & Resilience (Priority: P3)

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
- **SC-005**: All components are independently deployable and scalable (Except QuestDB which remains single-node for the initial version).
- **SC-006**: All components have unit tests with measurable code coverage.
- **SC-007**: All components have corresponding dockerfiles.

## Assumptions

- **Language Choice**: Implementation will be in Golang as per the project constitution.
- **Deployment**: Components will be containerized and deployed on Kubernetes (KIND for local testing).
- **Data Persistence**: MQ prioritizes in-memory storage for performance but performs periodic state snapshots to disk for crash recovery; full persistence is handled by the Collector.
- **Network Stack**: Custom binary protocol with length-prefixed framing over TCP.
binary protocol with length-prefixed framing over TCP.
