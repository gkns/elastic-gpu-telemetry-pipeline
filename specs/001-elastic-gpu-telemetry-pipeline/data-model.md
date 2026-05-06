# Data Model: Elastic GPU Telemetry Pipeline

**Phase**: 1 — Design  
**Date**: 2026-05-06

---

## 1. Wire Protocol: MQFrame

The atomic unit exchanged over TCP between all components and the MQ.

```go
// internal/protocol/frame.go

type MsgType uint8

const (
    MsgTypePublish   MsgType = 0x01 // Streamer → MQ: send telemetry
    MsgTypeACK       MsgType = 0x02 // Bidirectional: acknowledge receipt/processing
    MsgTypeNACK      MsgType = 0x03 // MQ → Streamer: requeue requested
    MsgTypeHello     MsgType = 0x04 // Connection handshake
    MsgTypeDeliver   MsgType = 0x05 // MQ → Collector: deliver telemetry
    MsgTypeHeartbeat MsgType = 0x06 // Keepalive ping (no body)
)

type NodeType uint8

const (
    NodeTypeStreamer  NodeType = 0x01
    NodeTypeCollector NodeType = 0x02
)

// Frame header layout (13 bytes):
// ┌─────────────┬──────────────┬──────────────┐
// │  Type (1B)  │  Length (4B) │  MsgID (8B)  │
// └─────────────┴──────────────┴──────────────┘
// Followed by `Length` bytes of body.

type Frame struct {
    Type   MsgType
    MsgID  uint64
    Body   []byte
}
```

### HelloBody (sent on new connection)

```go
type HelloBody struct {
    NodeType NodeType // 1 byte
    ID       string   // 2-byte length prefix + UTF-8 bytes
}
```

### PublishBody (Streamer → MQ, embedded in Frame.Body)

```go
type PublishBody struct {
    StreamerID string  // 2-byte length prefix + UTF-8
    SeqNum     uint64  // per-streamer monotonic sequence (8 bytes)
    Timestamp  int64   // Unix nanoseconds — system processing time injected by Streamer (8 bytes)
    Payload    []byte  // 4-byte length prefix + serialized TelemetryRecord (JSON)
}
```

**Validation rules**:
- `SeqNum` must be strictly increasing per `StreamerID` (MQ enforces; out-of-order frames are dropped and NACK sent).
- `Timestamp` is set by the Streamer to `time.Now().UnixNano()` at the moment the CSV row is read (not the timestamp in the CSV).
- `Payload` must be valid JSON-encoded `TelemetryRecord`.

### DeliverBody (MQ → Collector, same structure as PublishBody)

The MQ forwards the original `PublishBody` bytes verbatim inside a `MsgTypeDeliver` frame so the Collector can deserialize `TelemetryRecord` without re-encoding.

---

## 2. Domain Entity: TelemetryRecord

Parsed from the CSV input and serialized into `PublishBody.Payload`.

```go
// internal/telemetry/record.go

type TelemetryRecord struct {
    // Injected by Streamer — NOT from CSV timestamp column
    ProcessedAt time.Time `json:"processed_at"`

    MetricName string  `json:"metric_name"`
    GPUID      string  `json:"gpu_id"`
    Device     string  `json:"device"`
    UUID       string  `json:"uuid"`
    ModelName  string  `json:"model_name"`
    Hostname   string  `json:"hostname"`
    Container  string  `json:"container"`
    Pod        string  `json:"pod"`
    Namespace  string  `json:"namespace"`
    Value      float64 `json:"value"`
    LabelsRaw  string  `json:"labels_raw"`

    // Added by Streamer before publishing
    StreamerID string `json:"streamer_id"`
}
```

**CSV column mapping** (from sample data):
| CSV column | TelemetryRecord field |
|------------|-----------------------|
| `timestamp` | ignored (ProcessedAt is system time) |
| `metric_name` | MetricName |
| `gpu_id` | GPUID |
| `device` | Device |
| `uuid` | UUID |
| `modelName` | ModelName |
| `Hostname` | Hostname |
| `container` | Container |
| `pod` | Pod |
| `namespace` | Namespace |
| `value` | Value (parsed float64) |
| `labels_raw` | LabelsRaw |

---

## 3. MQ Internal State

### InFlightMessage

Tracks messages dispatched to a Collector but not yet ACK'd.

```go
// internal/mq/inflight.go

type InFlightMessage struct {
    MsgID       uint64
    CollectorID string
    StreamerID  string
    SeqNum      uint64
    SentAt      time.Time
    Retries     int
    Payload     []byte // original PublishBody bytes for redelivery
}
```

**Invariants**:
- At most one in-flight entry per `MsgID`.
- Redelivery after `5 * time.Second` timeout, up to 3 retries.
- After 3 retries with no live collectors, message is dropped and an error is logged (back-pressure applied upstream).

### StreamerQueue

Per-streamer message buffer between ingestion and routing.

```go
// internal/mq/queue.go

type StreamerQueue struct {
    StreamerID string
    Ch         chan *InFlightMessage // buffered, capacity 1000
    LastSeq    uint64                // for ordering validation
}
```

### CollectorConn

Live collector connection tracked by the MQ router.

```go
// internal/mq/router.go

type CollectorConn struct {
    ID   string
    Conn net.Conn
    Send chan *Frame // outbound frame queue for this collector
}
```

### WAL Entry (disk format)

```
| SeqNum (8B) | StreamerID len (2B) | StreamerID | PayloadLen (4B) | Payload |
```

Written append-only to `wal/<streamer_id>.wal`. Checkpoint file `wal/checkpoint.json`:
```json
{
  "ts": 1746518400,
  "streamers": {
    "streamer-1": { "last_seq": 4200, "wal_offset": 102400 }
  }
}
```

---

## 4. QuestDB Schema

Created at Collector startup if not present.

```sql
CREATE TABLE IF NOT EXISTS gpu_telemetry (
  ts          TIMESTAMP,      -- ProcessedAt (from TelemetryRecord)
  metric_name SYMBOL,
  gpu_id      SYMBOL,
  device      SYMBOL,
  uuid        SYMBOL,
  model_name  SYMBOL,
  hostname    SYMBOL,
  container   SYMBOL,
  pod         SYMBOL,
  namespace   SYMBOL,
  value       DOUBLE,
  labels_raw  STRING,
  streamer_id SYMBOL
) TIMESTAMP(ts) PARTITION BY DAY WAL;
```

**Design choices**:
- `SYMBOL`: dictionary-encoded, O(1) equality comparisons — ideal for low-cardinality string columns (metric names, GPU IDs, hostnames).
- `PARTITION BY DAY`: time-range queries scan only relevant partitions.
- `WAL` mode: crash-safe write-ahead log; QuestDB replays on restart.
- `ts` designated timestamp: enables QuestDB's native time-series optimizations (SAMPLE BY, LATEST ON).

### API Gateway Query Patterns

```sql
-- GET /api/v1/gpus
SELECT DISTINCT gpu_id FROM gpu_telemetry;

-- GET /api/v1/gpus/{id}/telemetry (paginated)
SELECT ts, metric_name, value, hostname, labels_raw
FROM gpu_telemetry
WHERE gpu_id = $1
ORDER BY ts DESC
LIMIT $2 OFFSET $3;

-- GET /api/v1/gpus/{id}/telemetry?start_time=...&end_time=...
SELECT ts, metric_name, value, hostname, labels_raw
FROM gpu_telemetry
WHERE gpu_id = $1
  AND ts BETWEEN $2 AND $3
ORDER BY ts DESC
LIMIT $4 OFFSET $5;
```

---

## 5. API Response Types

```go
// cmd/api-gateway/models.go

type GPU struct {
    ID string `json:"id"`
}

type TelemetryEntry struct {
    Timestamp  time.Time `json:"timestamp"`
    MetricName string    `json:"metric_name"`
    Value      float64   `json:"value"`
    Hostname   string    `json:"hostname"`
    LabelsRaw  string    `json:"labels_raw"`
}

type TelemetryPage struct {
    GPUID   string           `json:"gpu_id"`
    Page    int              `json:"page"`
    Size    int              `json:"size"`
    Total   int              `json:"total,omitempty"`
    Entries []TelemetryEntry `json:"entries"`
}
```

---

## 6. State Transitions

### MQ Message Lifecycle

```
[RECEIVED from Streamer]
        │ ACK → Streamer
        ▼
[QUEUED in StreamerQueue.Ch]
        │
        ▼ (router dequeues)
[IN-FLIGHT] ──── 5s timeout ──► [REDELIVERED] ──── 3 retries ──► [DROPPED]
        │
        │ ACK from Collector
        ▼
[ACKNOWLEDGED / DONE]
```

### Collector Connect/Disconnect

```
[CONNECTING] → HELLO handshake → [ACTIVE]
                                      │
                            ┌─────────┴────────┐
                       graceful              crash/drop
                            │                   │
                       [DRAINING]         [DISCONNECTED]
                            │                   │
                       ACK pending         5s timeout
                            └────────┬──────────┘
                                     ▼
                              router rehashes affected
                              streamers to remaining
                              live collectors
```

---

## 7. Configuration Types

Each component reads from environment variables.

```go
// internal/config/config.go

type MQConfig struct {
    ListenAddr      string        // default ":7000"
    SnapshotInterval time.Duration // default 30s
    WALDir          string        // default "./wal"
    InFlightTimeout time.Duration // default 5s
    MaxRetries      int           // default 3
    BufferCap       int           // default 1000 per streamer
}

type StreamerConfig struct {
    MQAddr      string // default "localhost:7000"
    StreamerID  string // default hostname
    CSVPath     string // required
    SendRate    time.Duration // delay between sends; 0 = as fast as possible
}

type CollectorConfig struct {
    MQAddr         string // default "localhost:7000"
    CollectorID    string // default hostname
    QuestDBILPAddr string // default "localhost:9009"
    QuestDBPGAddr  string // default "localhost:8812" (for table creation)
    WorkerCount    int    // default 4 (parallel ILP writers)
}

type APIGatewayConfig struct {
    ListenAddr    string // default ":8080"
    QuestDBPGAddr string // default "localhost:8812"
    PageSizeMax   int    // default 1000
}
```
