package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/questdb"
	"github.com/stretchr/testify/assert"
)

type errQuerier struct{}

func (e *errQuerier) ListGPUs(_ context.Context) ([]string, error) {
	return nil, errors.New("db error")
}
func (e *errQuerier) GetTelemetry(_ context.Context, _ string, _, _ int) (*questdb.TelemetryPage, error) {
	return nil, errors.New("db error")
}
func (e *errQuerier) GetTelemetryRange(_ context.Context, _ string, _, _ time.Time, _, _ int) (*questdb.TelemetryPage, error) {
	return nil, errors.New("db error")
}

func TestListGPUs_DBError_500(t *testing.T) {
	h := newTestHandlers(&errQuerier{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus", nil)
	w := httptest.NewRecorder()
	h.ListGPUs(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetGPUTelemetry_DBError_500(t *testing.T) {
	h := newTestHandlers(&errQuerier{})
	r := chi.NewRouter()
	r.Get("/api/v1/gpus/{id}/telemetry", h.GetGPUTelemetry)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus/gpu-1/telemetry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetGPUTelemetry_RangeDBError_500(t *testing.T) {
	h := newTestHandlers(&errQuerier{})
	r := chi.NewRouter()
	r.Get("/api/v1/gpus/{id}/telemetry", h.GetGPUTelemetry)
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/gpus/gpu-1/telemetry?start_time=2025-07-18T20:00:00Z&end_time=2025-07-18T21:00:00Z", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
