package config

import (
	"os"
	"strconv"
	"time"
)

type MQConfig struct {
	ListenAddr       string
	SnapshotInterval time.Duration
	WALDir           string
	InFlightTimeout  time.Duration
	MaxRetries       int
	BufferCap        int
}

type StreamerConfig struct {
	MQAddr     string
	StreamerID string
	CSVPath    string
	SendRate   time.Duration
}

type CollectorConfig struct {
	MQAddr         string
	CollectorID    string
	QuestDBILPAddr string
	QuestDBPGAddr  string
	WorkerCount    int
}

type APIGatewayConfig struct {
	ListenAddr    string
	QuestDBPGAddr string
	PageSizeMax   int
}

func LoadMQConfig() MQConfig {
	return MQConfig{
		ListenAddr:       getEnv("MQ_LISTEN_ADDR", ":7000"),
		SnapshotInterval: getDuration("MQ_SNAPSHOT_INTERVAL", 30*time.Second),
		WALDir:           getEnv("MQ_WAL_DIR", "./wal"),
		InFlightTimeout:  getDuration("MQ_INFLIGHT_TIMEOUT", 5*time.Second),
		MaxRetries:       getInt("MQ_MAX_RETRIES", 3),
		BufferCap:        getInt("MQ_BUFFER_CAP", 1000),
	}
}

func LoadStreamerConfig() StreamerConfig {
	id := getEnv("STREAMER_ID", "")
	if id == "" {
		id, _ = os.Hostname()
	}
	return StreamerConfig{
		MQAddr:     getEnv("STREAMER_MQ_ADDR", "localhost:7000"),
		StreamerID: id,
		CSVPath:    getEnv("STREAMER_CSV_PATH", ""),
		SendRate:   getDuration("STREAMER_SEND_RATE", 0),
	}
}

func LoadCollectorConfig() CollectorConfig {
	id := getEnv("COLLECTOR_ID", "")
	if id == "" {
		id, _ = os.Hostname()
	}
	return CollectorConfig{
		MQAddr:         getEnv("COLLECTOR_MQ_ADDR", "localhost:7000"),
		CollectorID:    id,
		QuestDBILPAddr: getEnv("COLLECTOR_QUESTDB_ILP_ADDR", "localhost:9009"),
		QuestDBPGAddr:  getEnv("COLLECTOR_QUESTDB_PG_ADDR", "localhost:8812"),
		WorkerCount:    getInt("COLLECTOR_WORKERS", 4),
	}
}

func LoadAPIGatewayConfig() APIGatewayConfig {
	return APIGatewayConfig{
		ListenAddr:    getEnv("GATEWAY_LISTEN_ADDR", ":8080"),
		QuestDBPGAddr: getEnv("GATEWAY_QUESTDB_PG_ADDR", "localhost:8812"),
		PageSizeMax:   getInt("GATEWAY_PAGE_SIZE_MAX", 1000),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
