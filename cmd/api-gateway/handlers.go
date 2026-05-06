package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/questdb"
)

// GPUQuerier abstracts QuestDB access, enabling mock injection in tests.
type GPUQuerier interface {
	ListGPUs(ctx context.Context) ([]string, error)
	GetTelemetry(ctx context.Context, gpuID string, page, pageSize int) (*questdb.TelemetryPage, error)
	GetTelemetryRange(ctx context.Context, gpuID string, start, end time.Time, page, pageSize int) (*questdb.TelemetryPage, error)
}

// Handlers holds the shared querier and page-size limit.
type Handlers struct {
	q           GPUQuerier
	pageSizeMax int
}

// ListGPUs returns all GPU IDs with available telemetry.
//
//	@Summary		List all GPUs with available telemetry data
//	@Tags			telemetry
//	@Produce		json
//	@Success		200	{array}		GPU
//	@Failure		500	{object}	ErrorResponse
//	@Router			/api/v1/gpus [get]
func (h *Handlers) ListGPUs(w http.ResponseWriter, r *http.Request) {
	gpus, err := h.q.ListGPUs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	result := make([]GPU, len(gpus))
	for i, id := range gpus {
		result[i] = GPU{ID: id}
	}
	writeJSON(w, http.StatusOK, result)
}

// GetGPUTelemetry returns paginated telemetry entries for a specific GPU.
//
//	@Summary		Get paginated telemetry entries for a specific GPU
//	@Tags			telemetry
//	@Produce		json
//	@Param			id			path		string	true	"GPU UUID"
//	@Param			start_time	query		string	false	"ISO 8601 start time"
//	@Param			end_time	query		string	false	"ISO 8601 end time"
//	@Param			page		query		int		false	"Page number (1-indexed)"	default(1)
//	@Param			page_size	query		int		false	"Entries per page (max 1000)"	default(100)
//	@Success		200			{object}	TelemetryPage
//	@Failure		400			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/api/v1/gpus/{id}/telemetry [get]
func (h *Handlers) GetGPUTelemetry(w http.ResponseWriter, r *http.Request) {
	gpuID := chi.URLParam(r, "id")
	q := r.URL.Query()

	page, err := parseIntParam(q.Get("page"), 1)
	if err != nil || page < 1 {
		writeError(w, http.StatusBadRequest, "invalid page")
		return
	}
	pageSize, err := parseIntParam(q.Get("page_size"), 100)
	if err != nil || pageSize < 1 {
		writeError(w, http.StatusBadRequest, "invalid page_size")
		return
	}
	if pageSize > h.pageSizeMax {
		pageSize = h.pageSizeMax
	}

	startStr := q.Get("start_time")
	endStr := q.Get("end_time")

	var result *questdb.TelemetryPage
	if startStr != "" && endStr != "" {
		start, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid start_time")
			return
		}
		end, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid end_time")
			return
		}
		result, err = h.q.GetTelemetryRange(r.Context(), gpuID, start, end, page, pageSize)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
	} else {
		result, err = h.q.GetTelemetry(r.Context(), gpuID, page, pageSize)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
	}

	if result == nil || (page == 1 && len(result.Entries) == 0) {
		writeError(w, http.StatusNotFound, "GPU not found")
		return
	}

	// Map questdb.TelemetryPage to local TelemetryPage.
	out := TelemetryPage{
		GPUID:   result.GPUID,
		Page:    result.Page,
		Size:    result.Size,
		Total:   result.Total,
		Entries: make([]TelemetryEntry, len(result.Entries)),
	}
	for i, e := range result.Entries {
		out.Entries[i] = TelemetryEntry{
			Timestamp:  e.Timestamp,
			MetricName: e.MetricName,
			Value:      e.Value,
			Hostname:   e.Hostname,
			LabelsRaw:  e.LabelsRaw,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// HealthCheck returns service health status.
//
//	@Summary		Health check
//	@Tags			ops
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Router			/healthz [get]
func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, ErrorResponse{Error: msg})
}

func parseIntParam(s string, def int) (int, error) {
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	return n, err
}
