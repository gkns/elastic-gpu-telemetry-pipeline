# Quickstart: Elastic GPU Telemetry Pipeline

## Local Setup (KIND)

1. **Initialize KIND cluster**:
   ```bash
   make kind-setup
   ```

2. **Deploy QuestDB**:
   ```bash
   make deploy-questdb
   ```

3. **Build and Deploy Pipeline**:
   ```bash
   make build && make deploy
   ```

4. **Verify Deployment**:
   ```bash
   kubectl get pods
   # Should see mq, streamer, collector, and api-gateway
   ```

## Using the API

1. **Port-forward API Gateway**:
   ```bash
   kubectl port-forward svc/api-gateway 8080:8080
   ```

2. **List GPUs**:
   ```bash
   curl http://localhost:8080/api/v1/gpus
   ```

3. **Get Telemetry**:
   ```bash
   curl "http://localhost:8080/api/v1/gpus/{id}/telemetry?page_size=10"
   ```

## Testing

Run all unit tests with coverage:
```bash
make test
```
