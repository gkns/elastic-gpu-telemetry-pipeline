package config_test

import (
	"testing"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadMQConfig_Defaults(t *testing.T) {
	t.Setenv("MQ_LISTEN_ADDR", "")
	t.Setenv("MQ_WAL_DIR", "")
	t.Setenv("MQ_SNAPSHOT_INTERVAL", "")
	t.Setenv("MQ_INFLIGHT_TIMEOUT", "")
	t.Setenv("MQ_MAX_RETRIES", "")
	t.Setenv("MQ_BUFFER_CAP", "")

	cfg := config.LoadMQConfig()
	assert.Equal(t, ":7000", cfg.ListenAddr)
	assert.Equal(t, "./wal", cfg.WALDir)
	assert.Equal(t, 30*time.Second, cfg.SnapshotInterval)
	assert.Equal(t, 5*time.Second, cfg.InFlightTimeout)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 1000, cfg.BufferCap)
}

func TestLoadMQConfig_EnvOverrides(t *testing.T) {
	t.Setenv("MQ_LISTEN_ADDR", ":9000")
	t.Setenv("MQ_WAL_DIR", "/tmp/wal")
	t.Setenv("MQ_SNAPSHOT_INTERVAL", "60s")
	t.Setenv("MQ_INFLIGHT_TIMEOUT", "10s")
	t.Setenv("MQ_MAX_RETRIES", "5")
	t.Setenv("MQ_BUFFER_CAP", "500")

	cfg := config.LoadMQConfig()
	assert.Equal(t, ":9000", cfg.ListenAddr)
	assert.Equal(t, "/tmp/wal", cfg.WALDir)
	assert.Equal(t, 60*time.Second, cfg.SnapshotInterval)
	assert.Equal(t, 10*time.Second, cfg.InFlightTimeout)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 500, cfg.BufferCap)
}

func TestLoadStreamerConfig_Defaults(t *testing.T) {
	t.Setenv("STREAMER_MQ_ADDR", "")
	t.Setenv("STREAMER_ID", "test-host")
	t.Setenv("STREAMER_CSV_PATH", "")
	t.Setenv("STREAMER_SEND_RATE", "")

	cfg := config.LoadStreamerConfig()
	assert.Equal(t, "localhost:7000", cfg.MQAddr)
	assert.Equal(t, "test-host", cfg.StreamerID)
	assert.Equal(t, time.Duration(0), cfg.SendRate)
}

func TestLoadStreamerConfig_EnvOverrides(t *testing.T) {
	t.Setenv("STREAMER_MQ_ADDR", "mq:7000")
	t.Setenv("STREAMER_ID", "s-1")
	t.Setenv("STREAMER_CSV_PATH", "/data/test.csv")
	t.Setenv("STREAMER_SEND_RATE", "10ms")

	cfg := config.LoadStreamerConfig()
	assert.Equal(t, "mq:7000", cfg.MQAddr)
	assert.Equal(t, "s-1", cfg.StreamerID)
	assert.Equal(t, "/data/test.csv", cfg.CSVPath)
	assert.Equal(t, 10*time.Millisecond, cfg.SendRate)
}

func TestLoadCollectorConfig_Defaults(t *testing.T) {
	t.Setenv("COLLECTOR_MQ_ADDR", "")
	t.Setenv("COLLECTOR_ID", "col-1")
	t.Setenv("COLLECTOR_QUESTDB_ILP_ADDR", "")
	t.Setenv("COLLECTOR_QUESTDB_PG_ADDR", "")
	t.Setenv("COLLECTOR_WORKERS", "")

	cfg := config.LoadCollectorConfig()
	assert.Equal(t, "localhost:7000", cfg.MQAddr)
	assert.Equal(t, "localhost:9009", cfg.QuestDBILPAddr)
	assert.Equal(t, "localhost:8812", cfg.QuestDBPGAddr)
	assert.Equal(t, 4, cfg.WorkerCount)
}

func TestLoadAPIGatewayConfig_Defaults(t *testing.T) {
	t.Setenv("GATEWAY_LISTEN_ADDR", "")
	t.Setenv("GATEWAY_QUESTDB_PG_ADDR", "")
	t.Setenv("GATEWAY_PAGE_SIZE_MAX", "")

	cfg := config.LoadAPIGatewayConfig()
	assert.Equal(t, ":8080", cfg.ListenAddr)
	assert.Equal(t, "localhost:8812", cfg.QuestDBPGAddr)
	assert.Equal(t, 1000, cfg.PageSizeMax)
}

func TestLoadAPIGatewayConfig_EnvOverrides(t *testing.T) {
	t.Setenv("GATEWAY_LISTEN_ADDR", ":9090")
	t.Setenv("GATEWAY_QUESTDB_PG_ADDR", "questdb:8812")
	t.Setenv("GATEWAY_PAGE_SIZE_MAX", "500")

	cfg := config.LoadAPIGatewayConfig()
	assert.Equal(t, ":9090", cfg.ListenAddr)
	assert.Equal(t, "questdb:8812", cfg.QuestDBPGAddr)
	assert.Equal(t, 500, cfg.PageSizeMax)
}
