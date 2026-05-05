# Data Model: Custom Messaging Queue System

## Entities

### TelemetryMessage
Represents the data packet flowing through the pipeline.
- `MessageID` (UUID): Unique identifier for deduplication and ACK tracking.
- `StreamerID` (UUID): Identifies the source streamer to ensure per-streamer ordering.
- `Timestamp` (RFC3339): When the telemetry was recorded.
- `Payload` (Binary/String): The raw CSV line or parsed telemetry metrics.
- `Status`: (Pending, Dispatched, Acknowledged, Failed).

### Node
Represents a connected component in the network.
- `NodeID` (UUID): Unique identifier.
- `Type`: (Streamer, Collector).
- `Connection`: (TCP Socket).
- `LastHeartbeat`: (Timestamp).

### QueueState
In-memory representation of the MQ.
- `Buffer`: (FIFO Queue of TelemetryMessage).
- `UnackedMessages`: (Map of MessageID -> {CollectorID, Expiry}).
- `StreamerOrderMap`: (Map of StreamerID -> LastSequenceID).
