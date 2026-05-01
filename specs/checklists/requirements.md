## General
- All code should be in clean idiomatic golang.
- All modules should be independently deployable.
- All modules should be independently scalable.
- The custom message queue does not use any existing open-source message queue implementation.
- Ensure unit tests are written for all modules and they are run successfully via `make`.

## Telemetry Streamer
- Streams data continuously from CSV.
- Injects processing time as timestamp.

## API Gateway
- Has these APIs at the minimum : `GET /api/v1/gpus` which returns the paginated telemetry info for all GPUs available ordered by time, `GET / api/v1/gpus/{id}/telemetry` which return a paginated telemetry entries for a specific GPU, ordered by time, `GET / api/v1/gpus/{id}/telemetry?start_time=...&end_time=...` which returns a paginated telemetry entries for a specific GPU, within the specific time range and ordered by time.
- OpenAPI spec updates successfully via Makefile.
- The default page size for all paginated APIs are 100 entries.
- Ensure the OpenAPI spec is updated successfully via `Makefile`.

