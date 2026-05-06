package questdb

import (
	"context"
	"fmt"

	qdb "github.com/questdb/go-questdb-client/v3"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/telemetry"
)

// ILPWriter wraps the QuestDB go client for high-throughput ILP writes.
type ILPWriter struct {
	sender qdb.LineSender // LineSender is an interface in go-questdb-client/v3
}

// NewILPWriter creates an ILPWriter connected to the QuestDB ILP endpoint (port 9009).
func NewILPWriter(ctx context.Context, addr string) (*ILPWriter, error) {
	sender, err := qdb.NewLineSender(ctx, qdb.WithTcp(), qdb.WithAddress(addr))
	if err != nil {
		return nil, fmt.Errorf("ilp connect: %w", err)
	}
	return &ILPWriter{sender: sender}, nil
}

// Write sends one telemetry record to the gpu_telemetry table.
func (w *ILPWriter) Write(ctx context.Context, rec *telemetry.TelemetryRecord) error {
	return w.sender.Table("gpu_telemetry").
		Symbol("metric_name", rec.MetricName).
		Symbol("gpu_id", rec.GPUID).
		Symbol("device", rec.Device).
		Symbol("uuid", rec.UUID).
		Symbol("model_name", rec.ModelName).
		Symbol("hostname", rec.Hostname).
		Symbol("container", rec.Container).
		Symbol("pod", rec.Pod).
		Symbol("namespace", rec.Namespace).
		Symbol("streamer_id", rec.StreamerID).
		Float64Column("value", rec.Value).
		StringColumn("labels_raw", rec.LabelsRaw).
		At(ctx, rec.ProcessedAt)
}

// Flush sends all buffered rows to QuestDB.
func (w *ILPWriter) Flush(ctx context.Context) error {
	return w.sender.Flush(ctx)
}

// Close closes the underlying connection.
func (w *ILPWriter) Close(ctx context.Context) {
	w.sender.Close(ctx)
}
