# Quickstart: Custom MQ Telemetry Pipeline

## Prerequisites
- Docker & Kubernetes (KIND)
- Go 1.21+
- `make`

## Local Development (KIND)

1. **Setup Cluster**:
   ```bash
   make kind-setup
   ```

2. **Build Images**:
   ```bash
   make docker-build
   ```

3. **Deploy with Helm**:
   ```bash
   make deploy
   ```

4. **Verify Components**:
   ```bash
   kubectl get pods
   # Should see mq, streamer, and collector pods running
   ```

5. **View Logs**:
   ```bash
   kubectl logs -l app=collector
   ```

## Running Manually (Go)

1. **Start MQ**:
   ```bash
   go run mq/cmd/main.go
   ```

2. **Start Collector**:
   ```bash
   go run collector/cmd/main.go
   ```

3. **Start Streamer**:
   ```bash
   go run streamer/cmd/main.go --input data/metrics.csv
   ```
