# API Gateway Contract (v1)

## Base Path: `/api/v1`

### GET /gpus
Returns a list of all GPUs for which telemetry data is available.

**Response (200 OK)**:
```json
[
  {
    "id": "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
    "model": "NVIDIA H100 80GB HBM3",
    "hostname": "mtv5-dgx1-hgpu-031"
  }
]
```

### GET /gpus/{id}/telemetry
Returns paginated telemetry entries for a specific GPU, ordered by time.

**Query Parameters**:
- `page`: Page number (default: 1)
- `page_size`: Entries per page (default: 50)
- `start_time`: ISO8601 timestamp (optional)
- `end_time`: ISO8601 timestamp (optional)

**Response (200 OK)**:
```json
{
  "gpu_id": "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
  "data": [
    {
      "timestamp": "2025-07-18T20:42:34Z",
      "metric": "DCGM_FI_DEV_GPU_UTIL",
      "value": 45.0
    }
  ],
  "pagination": {
    "current_page": 1,
    "total_pages": 10,
    "total_entries": 500
  }
}
```
