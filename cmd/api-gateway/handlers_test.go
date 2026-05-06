package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/questdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQuerier implements GPUQuerier for testing.
type mockQuerier struct {
	gpus         []string
	page         *questdb.TelemetryPage
	rangePageFn  func(gpuID string, start, end time.Time, page, pageSize int) *questdb.TelemetryPage
	queryRangeCalled bool
}

func (m *mockQuerier) ListGPUs(_ context.Context) ([]string, error) {
	return m.gpus, nil
}

func (m *mockQuerier) GetTelemetry(_ context.Context, gpuID string, pg, ps int) (*questdb.TelemetryPage, error) {
	if m.page != nil {
		return m.page, nil
	}
	return &questdb.TelemetryPage{GPUID: gpuID, Page: pg, Entries: []questdb.TelemetryEntry{}}, nil
}

func (m *mockQuerier) GetTelemetryRange(_ context.Context, gpuID string, start, end time.Time, pg, ps int) (*questdb.TelemetryPage, error) {
	m.queryRangeCalled = true
	if m.rangePageFn != nil {
		return m.rangePageFn(gpuID, start, end, pg, ps), nil
	}
	return &questdb.TelemetryPage{GPUID: gpuID, Page: pg, Entries: []questdb.TelemetryEntry{}}, nil
}

func newTestHandlers(q GPUQuerier) *Handlers {
	return &Handlers{q: q, pageSizeMax: 1000}
}

func TestListGPUs_200(t *testing.T) {
	q := &mockQuerier{gpus: []string{"gpu-1", "gpu-2"}}
	h := newTestHandlers(q)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus", nil)
	w := httptest.NewRecorder()
	h.ListGPUs(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []GPU
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 2)
	assert.Equal(t, "gpu-1", result[0].ID)
}

func TestGetGPUTelemetry_NoTimeParams_200(t *testing.T) {
	entry := questdb.TelemetryEntry{Timestamp: time.Now(), MetricName: "GPU_UTIL", Value: 42.0}
	q := &mockQuerier{page: &questdb.TelemetryPage{
		GPUID:   "gpu-1",
		Page:    1,
		Size:    1,
		Entries: []questdb.TelemetryEntry{entry},
	}}
	h := newTestHandlers(q)

	r := chi.NewRouter()
	r.Get("/api/v1/gpus/{id}/telemetry", h.GetGPUTelemetry)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus/gpu-1/telemetry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page TelemetryPage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, "gpu-1", page.GPUID)
	assert.Len(t, page.Entries, 1)
	assert.False(t, q.queryRangeCalled, "GetTelemetryRange must not be called without time params")
}

func TestGetGPUTelemetry_WithTimeParams_CallsRange(t *testing.T) {
	q := &mockQuerier{}
	h := newTestHandlers(q)

	r := chi.NewRouter()
	r.Get("/api/v1/gpus/{id}/telemetry", h.GetGPUTelemetry)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/gpus/gpu-1/telemetry?start_time=2025-07-18T20:00:00Z&end_time=2025-07-18T21:00:00Z", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Even though result is empty, it returns 404 (page 1, 0 entries).
	// The key assertion: GetTelemetryRange was called.
	assert.True(t, q.queryRangeCalled, "GetTelemetryRange must be called when time params provided")
}

func TestGetGPUTelemetry_InvalidPageSize_400(t *testing.T) {
	q := &mockQuerier{}
	h := newTestHandlers(q)

	r := chi.NewRouter()
	r.Get("/api/v1/gpus/{id}/telemetry", h.GetGPUTelemetry)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus/gpu-1/telemetry?page_size=0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetGPUTelemetry_UnknownGPU_404(t *testing.T) {
	q := &mockQuerier{page: &questdb.TelemetryPage{
		GPUID:   "unknown",
		Page:    1,
		Size:    0,
		Entries: []questdb.TelemetryEntry{},
	}}
	h := newTestHandlers(q)

	r := chi.NewRouter()
	r.Get("/api/v1/gpus/{id}/telemetry", h.GetGPUTelemetry)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus/unknown/telemetry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHealthCheck_200(t *testing.T) {
	h := newTestHandlers(&mockQuerier{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h.HealthCheck(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
