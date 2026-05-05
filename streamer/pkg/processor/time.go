package processor

import (
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/streamer/pkg/reader"
)

func InjectTimestamp(row *reader.TelemetryRow) {
	row.Timestamp = time.Now().Format(time.RFC3339)
}
