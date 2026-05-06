package telemetry

import "time"

// TelemetryRecord is parsed from a CSV row and carried through the pipeline.
// ProcessedAt is injected by the Streamer at read time (NOT from the CSV timestamp column).
type TelemetryRecord struct {
	ProcessedAt time.Time `json:"processed_at"`
	MetricName  string    `json:"metric_name"`
	GPUID       string    `json:"gpu_id"`
	Device      string    `json:"device"`
	UUID        string    `json:"uuid"`
	ModelName   string    `json:"model_name"`
	Hostname    string    `json:"hostname"`
	Container   string    `json:"container"`
	Pod         string    `json:"pod"`
	Namespace   string    `json:"namespace"`
	Value       float64   `json:"value"`
	LabelsRaw   string    `json:"labels_raw"`
	StreamerID  string    `json:"streamer_id"`
}
