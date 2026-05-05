package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/api-gateway/pkg/store"
	"github.com/go-chi/chi/v5"
)

type API struct {
	db *store.DB
}

func NewAPI(db *store.DB) *API {
	return &API{db: db}
}

// GetGPUs godoc
// @Summary List all GPUs
// @Description Get a list of all GPUs with telemetry data
// @Tags gpus
// @Produce json
// @Success 200 {array} store.GPU
// @Router /gpus [get]
func (a *API) GetGPUs(w http.ResponseWriter, r *http.Request) {
	gpus, err := a.db.GetGPUs(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(gpus)
}

// GetTelemetry godoc
// @Summary Get GPU telemetry
// @Description Get paginated telemetry entries for a specific GPU
// @Tags gpus
// @Produce json
// @Param id path string true "GPU UUID"
// @Param page query int false "Page number"
// @Param page_size query int false "Entries per page"
// @Param start_time query string false "Start time (ISO8601)"
// @Param end_time query string false "End time (ISO8601)"
// @Success 200 {object} TelemetryResponse
// @Router /gpus/{id}/telemetry [get]
func (a *API) GetTelemetry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 100
	}

	var startTime, endTime time.Time
	if st := r.URL.Query().Get("start_time"); st != "" {
		startTime, _ = time.Parse(time.RFC3339, st)
	}
	if et := r.URL.Query().Get("end_time"); et != "" {
		endTime, _ = time.Parse(time.RFC3339, et)
	}

	limit := pageSize
	offset := (page - 1) * pageSize

	data, total, err := a.db.GetTelemetry(r.Context(), id, startTime, endTime, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	resp := TelemetryResponse{
		GPUID: id,
		Data:  data,
		Pagination: Pagination{
			CurrentPage:  page,
			TotalPages:   totalPages,
			TotalEntries: total,
		},
	}
	json.NewEncoder(w).Encode(resp)
}

type TelemetryResponse struct {
	GPUID      string            `json:"gpu_id"`
	Data       []store.Telemetry `json:"data"`
	Pagination Pagination        `json:"pagination"`
}

type Pagination struct {
	CurrentPage  int `json:"current_page"`
	TotalPages   int `json:"total_pages"`
	TotalEntries int `json:"total_entries"`
}
