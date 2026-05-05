package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/streamer/pkg/reader"
)

func (db *DB) InsertTelemetry(ctx context.Context, row *reader.TelemetryRow) error {
	ts, err := time.Parse(time.RFC3339, row.Timestamp)
	if err != nil {
		return fmt.Errorf("failed to parse timestamp: %v", err)
	}

	val, err := strconv.ParseFloat(row.Value, 64)
	if err != nil {
		val = 0.0
	}

	// QuestDB uses 1000 * unix micro for timestamp in Postgres protocol? 
	// Actually, pgx will handle time.Time correctly if the column is TIMESTAMP.
	
	_, err = db.Pool.Exec(ctx,
		"INSERT INTO telemetries (timestamp, metric_name, gpu_id, device, uuid, model_name, hostname, value, labels_raw) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		ts, row.MetricName, row.GPUID, row.Device, row.UUID, row.ModelName, row.Hostname, val, row.LabelsRaw,
	)

	return err
}
