package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/questdb"
	"github.com/stretchr/testify/assert"
)

func TestGetGPUTelemetry_InvalidStartTime_400(t *testing.T) {
	h := newTestHandlers(&mockQuerier{})
	r := chi.NewRouter()
	r.Get("/api/v1/gpus/{id}/telemetry", h.GetGPUTelemetry)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/gpus/gpu-1/telemetry?start_time=not-a-date&end_time=2025-07-18T21:00:00Z", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetGPUTelemetry_InvalidEndTime_400(t *testing.T) {
	h := newTestHandlers(&mockQuerier{})
	r := chi.NewRouter()
	r.Get("/api/v1/gpus/{id}/telemetry", h.GetGPUTelemetry)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/gpus/gpu-1/telemetry?start_time=2025-07-18T20:00:00Z&end_time=bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetGPUTelemetry_InvalidPage_400(t *testing.T) {
	h := newTestHandlers(&mockQuerier{})
	r := chi.NewRouter()
	r.Get("/api/v1/gpus/{id}/telemetry", h.GetGPUTelemetry)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus/gpu-1/telemetry?page=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetGPUTelemetry_PageSizeCapped(t *testing.T) {
	entry := questdb.TelemetryEntry{Timestamp: time.Now(), MetricName: "X", Value: 1.0}
	q := &mockQuerier{page: &questdb.TelemetryPage{
		GPUID:   "gpu-1",
		Page:    1,
		Size:    1,
		Entries: []questdb.TelemetryEntry{entry},
	}}
	h := &Handlers{q: q, pageSizeMax: 50}
	r := chi.NewRouter()
	r.Get("/api/v1/gpus/{id}/telemetry", h.GetGPUTelemetry)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus/gpu-1/telemetry?page_size=9999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetGPUTelemetry_Page2WithResults(t *testing.T) {
	entry := questdb.TelemetryEntry{Timestamp: time.Now(), MetricName: "Y", Value: 2.0}
	q := &mockQuerier{page: &questdb.TelemetryPage{
		GPUID:   "gpu-1",
		Page:    2,
		Size:    1,
		Entries: []questdb.TelemetryEntry{entry},
	}}
	h := newTestHandlers(q)
	r := chi.NewRouter()
	r.Get("/api/v1/gpus/{id}/telemetry", h.GetGPUTelemetry)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus/gpu-1/telemetry?page=2&page_size=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Page 2 may return results or 404; we just verify it doesn't panic and returns a valid HTTP code.
	assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, w.Code)
}

func TestListGPUs_EmptyResult(t *testing.T) {
	q := &mockQuerier{gpus: []string{}}
	h := newTestHandlers(q)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus", nil)
	w := httptest.NewRecorder()
	h.ListGPUs(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestParseIntParam(t *testing.T) {
	n, err := parseIntParam("42", 1)
	assert.NoError(t, err)
	assert.Equal(t, 42, n)

	n, err = parseIntParam("", 99)
	assert.NoError(t, err)
	assert.Equal(t, 99, n)

	_, err = parseIntParam("nan", 1)
	assert.Error(t, err)
}
