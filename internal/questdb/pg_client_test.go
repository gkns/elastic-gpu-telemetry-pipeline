package questdb_test

import (
	"context"
	"testing"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/questdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRows implements the minimal pgRows interface for testing.
type mockRows struct {
	data  [][]any
	cur   int
	errAt int // row index to return error on Scan (-1 = never)
}

func newMockRows(data [][]any) *mockRows {
	return &mockRows{data: data, cur: -1, errAt: -1}
}

func (m *mockRows) Next() bool {
	m.cur++
	return m.cur < len(m.data)
}

func (m *mockRows) Scan(dest ...any) error {
	if m.cur == m.errAt {
		return assert.AnError
	}
	row := m.data[m.cur]
	for i, d := range dest {
		switch v := d.(type) {
		case *time.Time:
			*v = row[i].(time.Time)
		case *string:
			*v = row[i].(string)
		case *float64:
			*v = row[i].(float64)
		}
	}
	return nil
}

func (m *mockRows) Err() error { return nil }

// TestTelemetryEntry_Scan ensures scanPage maps columns correctly.
func TestTelemetryEntry_FieldMapping(t *testing.T) {
	ts := time.Date(2025, 7, 18, 20, 42, 34, 0, time.UTC)
	rows := newMockRows([][]any{
		{ts, "DCGM_FI_DEV_GPU_UTIL", 87.5, "host-1", `labels_raw_value`},
	})

	page, err := questdb.ScanPageForTest(rows, "gpu-1", 1, 100)
	require.NoError(t, err)
	require.Len(t, page.Entries, 1)

	e := page.Entries[0]
	assert.Equal(t, ts, e.Timestamp)
	assert.Equal(t, "DCGM_FI_DEV_GPU_UTIL", e.MetricName)
	assert.Equal(t, 87.5, e.Value)
	assert.Equal(t, "host-1", e.Hostname)
	assert.Equal(t, "labels_raw_value", e.LabelsRaw)
	assert.Equal(t, "gpu-1", page.GPUID)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 1, page.Size)
}

func TestTelemetryPage_EmptyResult(t *testing.T) {
	rows := newMockRows(nil)
	page, err := questdb.ScanPageForTest(rows, "gpu-x", 2, 50)
	require.NoError(t, err)
	assert.Empty(t, page.Entries)
	assert.Equal(t, 0, page.Size)
	assert.Equal(t, 2, page.Page)
}

// Verify PGQuerier interface is satisfied by PGClient at compile time.
var _ questdb.PGQuerier = (*questdb.PGClient)(nil)

// TestPGClient_InterfaceSatisfied ensures mock can satisfy PGQuerier.
type mockPGQuerier struct{}

func (m *mockPGQuerier) ListGPUs(_ context.Context) ([]string, error) {
	return []string{"gpu-1", "gpu-2"}, nil
}
func (m *mockPGQuerier) GetTelemetry(_ context.Context, gpuID string, _, _ int) (*questdb.TelemetryPage, error) {
	return &questdb.TelemetryPage{GPUID: gpuID, Page: 1, Entries: []questdb.TelemetryEntry{}}, nil
}
func (m *mockPGQuerier) GetTelemetryRange(_ context.Context, gpuID string, _, _ time.Time, _, _ int) (*questdb.TelemetryPage, error) {
	return &questdb.TelemetryPage{GPUID: gpuID, Page: 1, Entries: []questdb.TelemetryEntry{}}, nil
}

var _ questdb.PGQuerier = (*mockPGQuerier)(nil)
