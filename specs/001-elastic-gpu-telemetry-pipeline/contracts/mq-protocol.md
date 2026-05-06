# MQ Binary Wire Protocol Specification

**Version**: 1.0  
**Transport**: TCP  
**Byte order**: Big-endian

---

## Frame Format

Every message exchanged between the MQ and any client (Streamer or Collector) is wrapped in a frame.

```
┌─────────────┬──────────────┬──────────────┬──────────────────────┐
│  Type (1B)  │  Length (4B) │  MsgID (8B)  │  Body (Length bytes) │
└─────────────┴──────────────┴──────────────┴──────────────────────┘
Total header size: 13 bytes
```

| Field  | Size | Description |
|--------|------|-------------|
| Type   | 1 B  | Message type constant (see below) |
| Length | 4 B  | Number of bytes in Body (uint32 big-endian); 0 if no body |
| MsgID  | 8 B  | Unique message identifier (uint64 big-endian); 0 for control frames |
| Body   | N B  | Type-specific payload |

---

## Message Types

| Hex  | Name      | Direction           | Body |
|------|-----------|---------------------|------|
| 0x01 | PUBLISH   | Streamer → MQ       | PublishBody |
| 0x02 | ACK       | Bidirectional       | ACKBody |
| 0x03 | NACK      | MQ → Streamer       | NACKBody |
| 0x04 | HELLO     | Client → MQ         | HelloBody |
| 0x05 | DELIVER   | MQ → Collector      | PublishBody (forwarded verbatim) |
| 0x06 | HEARTBEAT | Bidirectional       | empty |

---

## Body Schemas

### HELLO Body

Sent immediately after TCP connection is established. Must be the first frame from any client.

```
┌──────────────┬───────────────────────────────────────────────────┐
│ NodeType(1B) │ IDLen(2B) │ ID(IDLen bytes)                       │
└──────────────┴───────────────────────────────────────────────────┘
```

| Field    | Type    | Values |
|----------|---------|--------|
| NodeType | uint8   | 0x01 = Streamer, 0x02 = Collector |
| IDLen    | uint16  | Length of ID string in bytes |
| ID       | []byte  | UTF-8 node identifier (hostname + random suffix recommended) |

The MQ responds with an ACK (MsgID=0) on success, or closes the connection on error.

---

### PUBLISH Body (Streamer → MQ)

```
┌───────────────────────────────────────────────────────────────────────┐
│ StreamerIDLen(2B) │ StreamerID(N) │ SeqNum(8B) │ Ts(8B) │ PayLen(4B) │ Payload(N) │
└───────────────────────────────────────────────────────────────────────┘
```

| Field       | Type   | Description |
|-------------|--------|-------------|
| StreamerIDLen | uint16 | Length of StreamerID in bytes |
| StreamerID  | []byte | UTF-8 streamer identifier |
| SeqNum      | uint64 | Monotonically increasing per StreamerID; starts at 1 |
| Ts          | int64  | System processing time (Unix nanoseconds) — injected by Streamer at read time |
| PayLen      | uint32 | Length of Payload |
| Payload     | []byte | JSON-encoded TelemetryRecord |

**MQ response**: ACK with the same MsgID on success; NACK if SeqNum is out of order or buffer is full.

---

### ACK Body

```
┌────────────────┐
│ AckMsgID (8B)  │
└────────────────┘
```

| Field    | Type   | Description |
|----------|--------|-------------|
| AckMsgID | uint64 | MsgID of the frame being acknowledged |

Used by:
- MQ → Streamer: acknowledges receipt of a PUBLISH frame.
- Collector → MQ: acknowledges successful processing of a DELIVER frame.
- MQ → Client: acknowledges HELLO handshake (AckMsgID = 0).

---

### NACK Body

```
┌────────────────┬───────────────┐
│ NackMsgID (8B) │ ReasonCode(1B) │
└────────────────┴───────────────┘
```

| Field      | Type   | Description |
|------------|--------|-------------|
| NackMsgID  | uint64 | MsgID of the rejected frame |
| ReasonCode | uint8  | 0x01=BufferFull, 0x02=OutOfOrder, 0x03=InternalError |

On receiving NACK, the Streamer MUST NOT advance its SeqNum and SHOULD retry after a backoff.

---

### DELIVER Body

Identical in encoding to PUBLISH Body. The MQ forwards the original PUBLISH body bytes verbatim inside a DELIVER frame, using a new MQ-assigned MsgID.

---

### HEARTBEAT

No body (Length = 0, MsgID = 0). Both sides send a heartbeat every 10 seconds to detect dead connections. A connection is considered dead if no frame is received within 30 seconds.

---

## Connection Lifecycle

```
Client                          MQ
  │──── TCP connect ────────────►│
  │──── HELLO ──────────────────►│
  │◄─── ACK(MsgID=0) ────────────│  (handshake complete)
  │                              │
  │  [Streamer only]             │
  │──── PUBLISH(MsgID=N) ───────►│
  │◄─── ACK(AckMsgID=N) ─────────│
  │                              │
  │  [Collector only]            │
  │◄─── DELIVER(MsgID=M) ────────│
  │──── ACK(AckMsgID=M) ────────►│
  │                              │
  │──── HEARTBEAT ──────────────►│  (every 10s)
  │◄─── HEARTBEAT ───────────────│
  │                              │
  │──── TCP close ──────────────►│  (graceful shutdown)
```

---

## Error Handling

| Scenario | MQ Behaviour |
|----------|-------------|
| Collector disconnects with in-flight messages | Re-deliver to next available collector after 5s timeout (up to 3 retries) |
| Streamer sends out-of-order SeqNum | NACK sent; frame dropped |
| Buffer full (streamer channel capacity 1000 exceeded) | NACK with ReasonCode=BufferFull; MQ blocks reading from that TCP connection (back-pressure) |
| No live collectors when delivering | Messages queued; delivery attempted when a collector reconnects |
| MQ restarts | Replays WAL from last checkpoint; re-delivers un-ACK'd messages |

---

## Routing

The MQ uses **sticky FNV-1a hash routing** by StreamerID:

```
collectorIndex = FNV1a(streamerID) mod len(activecollectors)
```

All messages from a given StreamerID are always routed to the same Collector (per-streamer ordering guarantee). On Collector disconnect, affected StreamerIDs are rehashed to the remaining live Collectors, and un-ACK'd in-flight messages for those streamers are redelivered.

---

## Defaults

| Parameter | Default |
|-----------|---------|
| MQ listen address | `:7000` |
| Heartbeat interval | 10s |
| Connection dead timeout | 30s |
| In-flight ACK timeout | 5s |
| Max redelivery retries | 3 |
| Per-streamer buffer capacity | 1000 messages |
| WAL snapshot interval | 30s |
