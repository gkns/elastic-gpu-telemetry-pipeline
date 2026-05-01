# Constitution: Telemetry Streamer

## Behavior
* The streamer must read data from the provided CSV file, which is under the folder <repo-root>/data/input_dcgm_metrics_20250718_134233.csv
* It must support looping the CSV data to simulate a continuous telemetry stream.
* The exact time a log is processed must be injected and considered as the timestamp for that telemetry point.

## Infrastructure
* The implementation must support dynamic scaling (up and down) of Streamer instances.