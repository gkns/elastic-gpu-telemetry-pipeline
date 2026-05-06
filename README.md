# Elastic GPU Telemetry Pipeline

An elastic, scalable, and stable telemetry pipeline for AI Clusters with a custom messaging queue.

## Architecture

- **Custom MQ**: High-performance TCP server with a binary protocol. Supports Round-robin distribution and back-pressure.
- **QuestDB**: Time-series database for telemetry persistence.
- **Telemetry Streamer**: Reads GPU metrics from CSV and streams them to the MQ.
- **Telemetry Collector**: Consumes messages from MQ and persists them to QuestDB.
- **API Gateway**: REST API for querying telemetry data with Swagger documentation.

## Prerequisites

- Go 1.21+
- Docker & Docker Compose
- KIND (for K8s deployment)
- Helm

## Getting Started

### Local Development (Docker Compose)

1. Start QuestDB:
   ```docker-compose -f deploy/questdb/docker-compose.yml up -d
   ```

2. Run MQ:
   ```go run mq/cmd/main.go
   ```

3. Run Collector:
   ```go run collector/cmd/main.go
   ```

4. Run Streamer:
   ```go run streamer/cmd/main.go --input data/metrics.csv
   ```

5. Run API Gateway:
   ```go run api-gateway/cmd/main.go
   ```

### Kubernetes Deployment (KIND)

1. Setup Cluster:
   ```make kind-setup
   ```

2. Build and Deploy:
   ```make build && make docker-build && make deploy
   ```

## API Documentation

Once the API Gateway is running, visit:
`http://localhost:8080/swagger/index.html`
