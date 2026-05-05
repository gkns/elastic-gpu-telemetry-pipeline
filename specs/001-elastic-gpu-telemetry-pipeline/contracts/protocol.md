# Custom MQ Binary Protocol Contract (v1)

## Overview
All messages use a fixed-length header followed by a variable-length payload. Multi-byte integers are encoded in **Big Endian**.

## Header Format (37 Bytes)

| Field | Size (Bytes) | Type | Description |
|---|---|---|---|
| Payload Length | 4 | uint32 | Size of the payload following the header. |
| Message Type | 1 | uint8 | 0x01: Streamer Hello, 0x02: Collector Hello, 0x03: Data, 0x04: ACK, 0x05: NACK |
| Message ID | 16 | UUID | Unique message identifier. |
| Streamer ID | 16 | UUID | Source identifier for ordering. |

## Message Types

### 0x01: Streamer Hello
Sent by the Streamer upon connection. Payload contains optional metadata (e.g., version).

### 0x02: Collector Hello
Sent by the Collector upon connection. Payload contains subscription filters (currently reserved).

### 0x03: Data
Telemetry data payload. Payload is the raw UTF-8 encoded CSV string.

### 0x04: ACK
Sent by the Collector to the MQ, then by the MQ to the Streamer. Confirms successful processing.

### 0x05: NACK
Indicates failure to process. MQ will re-queue the message.
