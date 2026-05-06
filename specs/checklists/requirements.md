## General
- All code should be in clean idiomatic golang.
- All modules should be independently deployable.
- All modules should be independently scalable.
- The custom message queue does not use any existing open-source message queue implementation.

## Custom Message Queue
- Accepts concurrent TCP connections from Streamers and Collectors.
- Routes messages from Streamers to available Collectors.
- Handles up to 10 concurrent Streamers and 10 concurrent Collectors.
- Implements an acknowledgment (ACK) mechanism.
- Ensure unit tests are written and are runnable using go test

## Telemetry Streamer
- Streams data continuously from CSV.
- Injects processing time as timestamp.
- Ensure unit tests are written

## Telemetry Collector
- Consumes messages from the Custom Message Queue.
- Sends ACKs back to the Message Queue upon receipt.
- Successfully parses the CSV payload and timestamp.
- Persists the parsed data into QuestDB using standard Postgres drivers.
- Ensure unit tests are written and are runnable using go test

## API Gateway
- Has these APIs at the minimum : 
  - `GET /api/v1/gpus` which returns a list of all GPUs for which telemetry data is available. 
  - `GET / api/v1/gpus/{id}/telemetry` which return a paginated telemetry entries for a specific GPU, ordered by time. 
  - `GET / api/v1/gpus/{id}/telemetry?start_time=...&end_time=...` which returns a paginated telemetry entries for a specific GPU, within the specific time range and ordered by time.
- OpenAPI spec updates successfully via Makefile.
- The default page size for all paginated APIs are 100 entries.
- Ensure the OpenAPI spec is updated successfully via `Makefile`.
- Ensure unit tests are written and are runnable using go test

