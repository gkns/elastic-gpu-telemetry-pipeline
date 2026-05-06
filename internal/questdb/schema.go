package questdb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS gpu_telemetry (
  ts          TIMESTAMP,
  metric_name SYMBOL,
  gpu_id      SYMBOL,
  device      SYMBOL,
  uuid        SYMBOL,
  model_name  SYMBOL,
  hostname    SYMBOL,
  container   SYMBOL,
  pod         SYMBOL,
  namespace   SYMBOL,
  value       DOUBLE,
  labels_raw  STRING,
  streamer_id SYMBOL
) TIMESTAMP(ts) PARTITION BY DAY WAL;`

// CreateTableIfNotExists connects to QuestDB over the PostgreSQL wire protocol
// and creates the gpu_telemetry table if it does not already exist.
func CreateTableIfNotExists(ctx context.Context, pgAddr string) error {
	conn, err := pgx.Connect(ctx, fmt.Sprintf("postgres://admin:quest@%s/qdb?sslmode=disable", pgAddr))
	if err != nil {
		return fmt.Errorf("connect to questdb: %w", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, createTableSQL); err != nil {
		return fmt.Errorf("create table: %w", err)
	}
	return nil
}
