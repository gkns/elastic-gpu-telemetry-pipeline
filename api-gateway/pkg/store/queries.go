package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func NewDB(connString string) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, err
	}
	return &DB{Pool: pool}, nil
}

type GPU struct {
	ID       string `json:"id"`
	Model    string `json:"model"`
	Hostname string `json:"hostname"`
}

type Telemetry struct {
	Timestamp time.Time `json:"timestamp"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
}

func (db *DB) GetGPUs(ctx context.Context) ([]GPU, error) {
	rows, err := db.Pool.Query(ctx, "SELECT DISTINCT uuid, model_name, hostname FROM telemetries")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gpus []GPU
	for rows.Next() {
		var g GPU
		if err := rows.Scan(&g.ID, &g.Model, &g.Hostname); err != nil {
			return nil, err
		}
		gpus = append(gpus, g)
	}
	return gpus, nil
}

func (db *DB) GetTelemetry(ctx context.Context, gpuID string, startTime, endTime time.Time, limit, offset int) ([]Telemetry, int, error) {
	query := "SELECT timestamp, metric_name, value FROM telemetries WHERE uuid = $1"
	args := []interface{}{gpuID}

	if !startTime.IsZero() {
		query += " AND timestamp >= $2"
		args = append(args, startTime)
	}
	if !endTime.IsZero() {
		idx := len(args) + 1
		query += fmt.Sprintf(" AND timestamp <= $%d", idx)
		args = append(args, endTime)
	}

	query += " ORDER BY timestamp DESC"
	
	// Count total for pagination
	var total int
	countQuery := "SELECT count(*) FROM (" + query + ") as sub"
	if err := db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	idx := len(args) + 1
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []Telemetry
	for rows.Next() {
		var t Telemetry
		if err := rows.Scan(&t.Timestamp, &t.Metric, &t.Value); err != nil {
			return nil, 0, err
		}
		results = append(results, t)
	}

	return results, total, nil
}
