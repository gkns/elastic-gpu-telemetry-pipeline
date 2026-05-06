package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/config"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/protocol"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/telemetry"
)

var msgIDCounter atomic.Uint64

func nextMsgID() uint64 { return msgIDCounter.Add(1) }

func main() {
	cfg := config.LoadStreamerConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	if cfg.CSVPath == "" {
		logger.Error("STREAMER_CSV_PATH is required")
		os.Exit(1)
	}

	conn := connectWithBackoff(cfg.MQAddr, logger)
	defer conn.Close()

	r := bufio.NewReader(conn)

	// HELLO handshake.
	hello := &protocol.HelloBody{NodeType: protocol.NodeTypeStreamer, ID: cfg.StreamerID}
	if err := protocol.WriteFrame(conn, &protocol.Frame{
		Type:  protocol.MsgTypeHello,
		MsgID: 0,
		Body:  hello.Marshal(),
	}); err != nil {
		logger.Error("send hello", "err", err)
		os.Exit(1)
	}
	f, err := protocol.ReadFrame(r)
	if err != nil || f.Type != protocol.MsgTypeACK {
		logger.Error("hello not acked")
		os.Exit(1)
	}

	// Heartbeat sender goroutine.
	go func() {
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for range tick.C {
			if err := protocol.WriteFrame(conn, protocol.NewHeartbeatFrame()); err != nil {
				return
			}
		}
	}()

	parser, err := telemetry.NewCSVParser(cfg.CSVPath)
	if err != nil {
		logger.Error("open csv", "err", err)
		os.Exit(1)
	}
	defer parser.Close()

	var seqNum uint64
	row := 0

	for {
		rec, err := parser.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Error("csv read", "err", err)
			break
		}
		rec.StreamerID = cfg.StreamerID
		row++

		payload, err := json.Marshal(rec)
		if err != nil {
			logger.Error("marshal record", "err", err)
			continue
		}

		seqNum++
		pb := &protocol.PublishBody{
			StreamerID: cfg.StreamerID,
			SeqNum:     seqNum,
			Timestamp:  rec.ProcessedAt.UnixNano(),
			Payload:    payload,
		}

		msgID := nextMsgID()
		frame := &protocol.Frame{
			Type:  protocol.MsgTypePublish,
			MsgID: msgID,
			Body:  pb.Marshal(),
		}

	retry:
		if err := protocol.WriteFrame(conn, frame); err != nil {
			logger.Error("write frame", "err", err)
			conn.Close()
			conn = connectWithBackoff(cfg.MQAddr, logger)
			r = bufio.NewReader(conn)
			goto retry
		}

		resp, err := protocol.ReadFrame(r)
		if err != nil {
			logger.Error("read response", "err", err)
			conn.Close()
			conn = connectWithBackoff(cfg.MQAddr, logger)
			r = bufio.NewReader(conn)
			goto retry
		}

		switch resp.Type {
		case protocol.MsgTypeACK:
			logger.Debug("ack", "row", row, "seq", seqNum)
		case protocol.MsgTypeNACK:
			seqNum-- // do not advance SeqNum on NACK
			time.Sleep(100 * time.Millisecond)
			goto retry
		case protocol.MsgTypeHeartbeat:
			// Heartbeat received while waiting for ACK — loop back to read the actual ACK.
			goto readResp
		}

		if cfg.SendRate > 0 {
			time.Sleep(cfg.SendRate)
		}
		continue

	readResp:
		resp, err = protocol.ReadFrame(r)
		if err != nil {
			logger.Error("read response after heartbeat", "err", err)
			break
		}
		if resp.Type == protocol.MsgTypeACK {
			logger.Debug("ack", "row", row, "seq", seqNum)
		} else if resp.Type == protocol.MsgTypeNACK {
			seqNum--
			time.Sleep(100 * time.Millisecond)
			goto retry
		}

		if cfg.SendRate > 0 {
			time.Sleep(cfg.SendRate)
		}
	}

	logger.Info("streamer done", "rows_sent", row)
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
		logger.Warn("MQ connect failed, retrying", "addr", addr, "backoff", backoff, "err", fmt.Sprintf("%v", err))
		time.Sleep(backoff)
		backoff *= 2
		if backoff > 5*time.Second {
			backoff = 5 * time.Second
		}
	}
}
