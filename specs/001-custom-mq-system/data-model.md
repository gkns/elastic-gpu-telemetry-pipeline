# Data Model: Elastic GPU Telemetry Pipeline

## MQ Entities (In-Memory / Snapshot)

### Message
- `ID` (UUID): Unique message identifier.
- `StreamerID` (UUID): Identifier of the source streamer.
- `Timestamp` (RFC3339): Processing time injected by streamer.
- `Payload` (CSV Row): The raw telemetry data.

## QuestDB Schema (Telemetries Table)

| Column | Type | Description |
|---|---|---|
| `timestamp` | TIMESTAMP | Processing time (QuestDB designated timestamp) |
| `metric_name` | SYMBOL | Name of the DCGM metric |
| `gpu_id` | SYMBOL | ID of the GPU |
| `device` | SYMBOL | Device name |
| `uuid` | SYMBOL | GPU UUID |
| `model_name` | SYMBOL | GPU Model |
| `hostname` | SYMBOL | Source Hostname |
| `value` | DOUBLE | Metric value |
| `labels_raw` | STRING | Raw labels string for complex parsing |

## API Entities

### GPUInfo
- `ID`: GPU UUID.
- `Model`: Model name.
- `Hostname`: Hostname where the GPU is located.

### TelemetryEntry
- `Timestamp`: When it was processed.
- `Metric`: Metric name.
- `Value`: The numeric value.
