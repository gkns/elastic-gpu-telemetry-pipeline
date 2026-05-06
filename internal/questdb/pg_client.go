package questdb

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TelemetryEntry is a single row returned by the API Gateway.
type TelemetryEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	MetricName string    `json:"metric_name"`
	Value      float64   `json:"value"`
	Hostname   string    `json:"hostname"`
	LabelsRaw  string    `json:"labels_raw"`
}

// TelemetryPage is the paginated response for a GPU's telemetry.
type TelemetryPage struct {
	GPUID   string           `json:"gpu_id"`
	Page    int              `json:"page"`
	Size    int              `json:"size"`
	Total   int              `json:"total,omitempty"`
	Entries []TelemetryEntry `json:"entries"`
}

// PGQuerier abstracts the QuestDB pgx pool for testing.
type PGQuerier interface {
	ListGPUs(ctx context.Context) ([]string, error)
	GetTelemetry(ctx context.Context, gpuID string, page, pageSize int) (*TelemetryPage, error)
	GetTelemetryRange(ctx context.Context, gpuID string, start, end time.Time, page, pageSize int) (*TelemetryPage, error)
}

// PGClient implements PGQuerier over a real pgx/v5 pool.
type PGClient struct {
	pool *pgxpool.Pool
}

// NewPGClient creates a PGClient connected to QuestDB's PostgreSQL wire interface.
func NewPGClient(ctx context.Context, pgAddr string) (*PGClient, error) {
	dsn := fmt.Sprintf("postgres://admin:quest@%s/qdb?sslmode=disable", pgAddr)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgx pool: %w", err)
	}
	return &PGClient{pool: pool}, nil
}

// Close releases the connection pool.
func (c *PGClient) Close() {
	c.pool.Close()
}

// ListGPUs returns all distinct GPU IDs in the telemetry table.
func (c *PGClient) ListGPUs(ctx context.Context) ([]string, error) {
	rows, err := c.pool.Query(ctx, "SELECT DISTINCT gpu_id FROM gpu_telemetry")
	if err != nil {
		return nil, fmt.Errorf("list gpus: %w", err)
	}
	defer rows.Close()

	var gpus []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		gpus = append(gpus, id)
	}
	return gpus, rows.Err()
}

// GetTelemetry returns a page of telemetry entries for a GPU, ordered by ts DESC.
// QuestDB uses LIMIT lower,upper (row range) instead of standard LIMIT/OFFSET.
func (c *PGClient) GetTelemetry(ctx context.Context, gpuID string, page, pageSize int) (*TelemetryPage, error) {
	offset := (page - 1) * pageSize
	q := fmt.Sprintf(`SELECT ts, metric_name, value, hostname, labels_raw
		FROM gpu_telemetry
		WHERE gpu_id = $1
		ORDER BY ts DESC
		LIMIT %d,%d`, offset, offset+pageSize)

	rows, err := c.pool.Query(ctx, q, gpuID)
	if err != nil {
		return nil, fmt.Errorf("get telemetry: %w", err)
	}
	defer rows.Close()
	return scanPage(rows, gpuID, page, pageSize)
}

// GetTelemetryRange returns a page of telemetry entries within a time range, ordered by ts DESC.
func (c *PGClient) GetTelemetryRange(ctx context.Context, gpuID string, start, end time.Time, page, pageSize int) (*TelemetryPage, error) {
	offset := (page - 1) * pageSize
	q := fmt.Sprintf(`SELECT ts, metric_name, value, hostname, labels_raw
		FROM gpu_telemetry
		WHERE gpu_id = $1
		  AND ts BETWEEN $2 AND $3
		ORDER BY ts DESC
		LIMIT %d,%d`, offset, offset+pageSize)

	rows, err := c.pool.Query(ctx, q, gpuID, start, end)
	if err != nil {
		return nil, fmt.Errorf("get telemetry range: %w", err)
	}
	defer rows.Close()
	return scanPage(rows, gpuID, page, pageSize)
}

type pgRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanPage(rows pgRows, gpuID string, page, pageSize int) (*TelemetryPage, error) {
	p := &TelemetryPage{GPUID: gpuID, Page: page, Entries: []TelemetryEntry{}}
	for rows.Next() {
		var e TelemetryEntry
		if err := rows.Scan(&e.Timestamp, &e.MetricName, &e.Value, &e.Hostname, &e.LabelsRaw); err != nil {
			return nil, err
		}
		p.Entries = append(p.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	p.Size = len(p.Entries)
	return p, nil
}
