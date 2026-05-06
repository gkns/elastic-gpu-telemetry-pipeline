package main

import "time"

// GPU represents a GPU for which telemetry data is available.
// swagger:model GPU
type GPU struct {
	ID string `json:"id"`
}

// TelemetryEntry is a single telemetry data point.
// swagger:model TelemetryEntry
type TelemetryEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	MetricName string    `json:"metric_name"`
	Value      float64   `json:"value"`
	Hostname   string    `json:"hostname"`
	LabelsRaw  string    `json:"labels_raw"`
}

// TelemetryPage is a paginated collection of telemetry entries for a GPU.
// swagger:model TelemetryPage
type TelemetryPage struct {
	GPUID   string           `json:"gpu_id"`
	Page    int              `json:"page"`
	Size    int              `json:"size"`
	Total   int              `json:"total,omitempty"`
	Entries []TelemetryEntry `json:"entries"`
}

// ErrorResponse is the standard error envelope.
// swagger:model ErrorResponse
type ErrorResponse struct {
	Error string `json:"error"`
}
