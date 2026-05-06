package telemetry_test

import (
	"io"
	"testing"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const csvPath = "../../data/input_dcgm_metrics_20250718_134233.csv"

func TestCSVParser_FirstRows(t *testing.T) {
	p, err := telemetry.NewCSVParser(csvPath)
	require.NoError(t, err)
	defer p.Close()

	before := time.Now()
	rec, err := p.Next()
	after := time.Now()

	require.NoError(t, err)
	require.NotNil(t, rec)

	// ProcessedAt must be system time, not CSV timestamp.
	assert.True(t, rec.ProcessedAt.After(before) || rec.ProcessedAt.Equal(before))
	assert.True(t, rec.ProcessedAt.Before(after) || rec.ProcessedAt.Equal(after))

	assert.Equal(t, "DCGM_FI_DEV_GPU_UTIL", rec.MetricName)
	assert.Equal(t, "0", rec.GPUID)
	assert.Equal(t, float64(0), rec.Value)
}

func TestCSVParser_CSVTimestampIgnored(t *testing.T) {
	p, err := telemetry.NewCSVParser(csvPath)
	require.NoError(t, err)
	defer p.Close()

	rec, err := p.Next()
	require.NoError(t, err)

	// The CSV timestamp "2025-07-18T20:42:34Z" must NOT appear in ProcessedAt.
	csvTime := time.Date(2025, 7, 18, 20, 42, 34, 0, time.UTC)
	assert.NotEqual(t, csvTime, rec.ProcessedAt,
		"ProcessedAt must be system time, not the CSV timestamp column")
}

func TestCSVParser_ReadMultipleRows(t *testing.T) {
	p, err := telemetry.NewCSVParser(csvPath)
	require.NoError(t, err)
	defer p.Close()

	count := 0
	for {
		_, err := p.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		count++
		if count >= 3 {
			break
		}
	}
	assert.Equal(t, 3, count)
}

func TestCSVParser_EOFSignal(t *testing.T) {
	p, err := telemetry.NewCSVParser(csvPath)
	require.NoError(t, err)
	defer p.Close()

	var last error
	for {
		_, err := p.Next()
		if err != nil {
			last = err
			break
		}
	}
	assert.Equal(t, io.EOF, last)
}
