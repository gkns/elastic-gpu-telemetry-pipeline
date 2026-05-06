package questdb_test

import (
	"testing"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/questdb"
	"github.com/stretchr/testify/assert"
)

// TestCreateTableSQL_ContainsRequiredColumns verifies the DDL contains all
// expected columns. Full execution is tested in integration tests (requires QuestDB).
func TestCreateTableSQL_ContainsRequiredColumns(t *testing.T) {
	sql := questdb.GetCreateTableSQL()
	for _, want := range []string{
		"gpu_telemetry", "ts", "metric_name", "gpu_id", "device",
		"uuid", "model_name", "hostname", "container",
		"pod", "namespace", "value", "labels_raw", "streamer_id",
		"TIMESTAMP(ts)", "PARTITION BY DAY", "WAL",
	} {
		assert.Contains(t, sql, want, "DDL must contain column/keyword: %s", want)
	}
}
