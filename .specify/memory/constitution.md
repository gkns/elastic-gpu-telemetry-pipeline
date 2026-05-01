<!-- SPECKIT START -->
# Elastic GPU Telemetry Pipeline with Message Queue - Constitution


## 🎯 Project Goal
Design and implement an elastic, scalable, and stable telemetry pipeline for an AI Cluster with a custom messaging queue.
The system will process GPU telemetry data from CSV files and expose it via a REST API. A sample line with title and data from the telemetry data CSV is given below:

```timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_UTIL","0","nvidia0","GPU-5fd4f087-86f3-7a43-b711-4771313afc50","NVIDIA H100 80GB HBM3","mtv5-dgx1-hgpu-031","","","","0","DCGM_FI_DRIVER_VERSION=""535.129.03"",Hostname=""mtv5-dgx1-hgpu-031"",UUID=""GPU-5fd4f087-86f3-7a43-b711-4771313afc50"",__name__=""DCGM_FI_DEV_GPU_UTIL"",device=""nvidia0"",gpu=""0"",instance=""mtv5-dgx1-hgpu-031:9400"",job=""dgx_dcgm_exporter"",modelName=""NVIDIA H100 80GB HBM3"""
```

## 🏗️ Core Components / Modules
1. **Custom Messaging Queue**
   - Custom implementation (do NOT use ZeroMQ, RabbitMQ, Kafka, etc.).
   - Connects streamers and collectors.
   - Designed for scale, performance, and availability (up to 10 instances for streamer/collector).

2. **Telemetry Streamer**
   - Reads telemetry from CSV and streams it periodically over the custom message queue.
   - Process time = telemetry timestamp.
   - Dynamically scalable up/down.

3. **Telemetry Collector**
   - Consumes telemetry from the custom MQ, parses, and persists it.
   - Dynamically scalable up/down.

4. **API Gateway**
   - REST API exposing telemetry.
   - Auto-generated OpenAPI spec.
   - This API layer will be a separate module and independently deployable and scalable.

## 🏗️ Constraints for components / Modules
- Each of the components should be independently deployable and scalable.
- Each module should talk to other modules only through the custom MQ.
- Each of the components should have its corresponding dockerfile.

## 🛠️ Technology Stack
- **Programming Language**: Golang (clean, idiomatic)
- **Deployment**: Docker + Kubernetes
- **Deployment Tooling**: Helm
- **API Documentation**: OpenAPI (Swagger)
- **Build tooling**: `Makefile` for builds, code coverage, and OpenAPI spec generation

## Non-Functional Requirements
- **Code Quality**: Clean, maintainable systems-level code. Handle error paths and memory management gracefully. Also, log the errors clearly
- **Testing**: Mandatory unit tests with measurable code coverage via `Makefile`. (System tests are bonus).
- **Packaging & Deployment**: Dockerfiles for all applications (ustom Messaging Queue, ), and Helm charts for individual modules and an Umbrella chart for the entire stack on Kubernetes. For local testing a KIND K8s cluster will be used. Make sure it is deployable on KIND.

## General Constraints
- Whenever a change is requested in a module, the change should be implemented in that module only.


<!-- SPECKIT END -->
