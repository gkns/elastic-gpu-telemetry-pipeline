package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/config"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/protocol"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/questdb"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/telemetry"
)

// TelemetryWriter is the write interface used by ILP workers.
// Kept as an interface to allow mocking in tests.
type TelemetryWriter interface {
	Write(ctx context.Context, rec *telemetry.TelemetryRecord) error
	Flush(ctx context.Context) error
}

func main() {
	cfg := config.LoadCollectorConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx := context.Background()

	// Ensure the gpu_telemetry table exists before accepting any messages.
	if err := questdb.CreateTableIfNotExists(ctx, cfg.QuestDBPGAddr); err != nil {
		logger.Error("create table failed", "err", err)
		os.Exit(1)
	}

	// Pool of ILP writer goroutines.
	workCh := make(chan *telemetry.TelemetryRecord, cfg.WorkerCount*4)
	for i := 0; i < cfg.WorkerCount; i++ {
		go func(workerID int) {
			w, err := questdb.NewILPWriter(ctx, cfg.QuestDBILPAddr)
			if err != nil {
				logger.Error("ilp writer init failed", "worker", workerID, "err", err)
				return
			}
			defer w.Close(ctx)
			for rec := range workCh {
				writeWithRetry(ctx, w, rec, logger)
			}
		}(i)
	}

	conn := connectWithBackoff(cfg.MQAddr, logger)
	defer conn.Close()

	r := bufio.NewReader(conn)
	if err := doHandshake(conn, r, cfg.CollectorID, logger); err != nil {
		logger.Error("handshake failed", "err", err)
		os.Exit(1)
	}

	// Heartbeat sender.
	go func() {
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for range tick.C {
			if err := protocol.WriteFrame(conn, protocol.NewHeartbeatFrame()); err != nil {
				return
			}
		}
	}()

	// Throughput logger.
	var received atomic.Int64
	go func() {
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		for range tick.C {
			n := received.Swap(0)
			logger.Info("throughput", "rows_per_5s", n)
		}
	}()

	for {
		f, err := protocol.ReadFrame(r)
		if err != nil {
			logger.Error("read frame", "err", err)
			conn.Close()
			conn = connectWithBackoff(cfg.MQAddr, logger)
			r = bufio.NewReader(conn)
			if err2 := doHandshake(conn, r, cfg.CollectorID, logger); err2 != nil {
				logger.Error("reconnect handshake failed", "err", err2)
				continue
			}
			continue
		}

		switch f.Type {
		case protocol.MsgTypeDeliver:
			pb, err := protocol.UnmarshalPublishBody(f.Body)
			if err != nil {
				logger.Error("unmarshal publish body", "err", err)
				protocol.WriteFrame(conn, protocol.NewNACKFrame(f.MsgID, protocol.NACKReasonInternalError))
				continue
			}

			var rec telemetry.TelemetryRecord
			if err := json.Unmarshal(pb.Payload, &rec); err != nil {
				logger.Error("unmarshal telemetry record", "err", err)
				protocol.WriteFrame(conn, protocol.NewNACKFrame(f.MsgID, protocol.NACKReasonInternalError))
				continue
			}
			rec.ProcessedAt = time.Unix(0, pb.Timestamp).UTC()

			select {
			case workCh <- &rec:
			default:
				logger.Warn("worker channel full, dropping record", "msg_id", f.MsgID)
			}

			// ACK to MQ — the at-least-once guarantee is maintained by writeWithRetry.
			if err := protocol.WriteFrame(conn, protocol.NewACKFrame(f.MsgID)); err != nil {
				logger.Error("send ack", "err", err)
			}
			received.Add(1)

		case protocol.MsgTypeHeartbeat:
			// keepalive — no action needed

		default:
			logger.Warn("unexpected frame type", "type", fmt.Sprintf("0x%02x", f.Type))
		}
	}
}

func doHandshake(conn net.Conn, r *bufio.Reader, collectorID string, logger *slog.Logger) error {
	hello := &protocol.HelloBody{NodeType: protocol.NodeTypeCollector, ID: collectorID}
	if err := protocol.WriteFrame(conn, &protocol.Frame{
		Type:  protocol.MsgTypeHello,
		MsgID: 0,
		Body:  hello.Marshal(),
	}); err != nil {
		return fmt.Errorf("send hello: %w", err)
	}
	f, err := protocol.ReadFrame(r)
	if err != nil {
		return fmt.Errorf("read hello ack: %w", err)
	}
	if f.Type != protocol.MsgTypeACK {
		return fmt.Errorf("expected ACK, got 0x%02x", f.Type)
	}
	logger.Info("handshake complete", "id", collectorID)
	return nil
}

// writeWithRetry attempts to write to QuestDB up to 3 times with backoff.
// On permanent failure it logs — the MQ already ACKed the message so we log
// rather than NACK to avoid infinite redelivery loops for bad payloads.
func writeWithRetry(ctx context.Context, w TelemetryWriter, rec *telemetry.TelemetryRecord, logger *slog.Logger) {
	backoffs := []time.Duration{100 * time.Millisecond, 500 * time.Millisecond, time.Second}
	for i, b := range backoffs {
		if err := w.Write(ctx, rec); err != nil {
			logger.Warn("ilp write failed", "attempt", i+1, "err", err)
			time.Sleep(b)
			continue
		}
		if err := w.Flush(ctx); err != nil {
			logger.Warn("ilp flush failed", "attempt", i+1, "err", err)
			time.Sleep(b)
			continue
		}
		return
	}
	logger.Error("ilp write permanently failed after 3 retries", "metric", rec.MetricName, "gpu", rec.GPUID)
}

// connectWithBackoff dials the MQ with exponential backoff (max 5s between retries).
func connectWithBackoff(addr string, logger *slog.Logger) net.Conn {
	backoff := 200 * time.Millisecond
	for {
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			logger.Info("connected to MQ", "addr", addr)
			return conn
		}
		logger.Warn("MQ connect failed, retrying", "addr", addr, "backoff", backoff)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > 5*time.Second {
			backoff = 5 * time.Second
		}
	}
}
