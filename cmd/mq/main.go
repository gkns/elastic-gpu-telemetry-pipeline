package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/config"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/mq"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/protocol"
)

var msgIDCounter atomic.Uint64

func nextMsgID() uint64 {
	return msgIDCounter.Add(1)
}

func main() {
	cfg := config.LoadMQConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	wal, err := mq.NewWAL(cfg.WALDir)
	if err != nil {
		logger.Error("failed to init WAL", "err", err)
		os.Exit(1)
	}

	cp, err := wal.ReadCheckpoint()
	if err != nil {
		logger.Warn("could not read checkpoint, starting fresh", "err", err)
		cp, _ = wal.ReadCheckpoint()
	}

	router := &mq.Router{}
	tracker := mq.NewInFlightTracker()

	// Replay un-ACKed messages from WAL into a redelivery queue.
	replayMsgs, _ := wal.ReplayFrom(cp)
	replayCh := make(chan *mq.InFlightMessage, len(replayMsgs)+1)
	for _, m := range replayMsgs {
		m.MsgID = nextMsgID()
		replayCh <- m
	}
	close(replayCh)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		logger.Error("listen failed", "addr", cfg.ListenAddr, "err", err)
		os.Exit(1)
	}
	logger.Info("MQ listening", "addr", cfg.ListenAddr)

	// Per-streamer queues, protected by a mutex.
	var queuesMu sync.Mutex
	queues := make(map[string]*mq.StreamerQueue)

	getOrCreateQueue := func(streamerID string) *mq.StreamerQueue {
		queuesMu.Lock()
		defer queuesMu.Unlock()
		if q, ok := queues[streamerID]; ok {
			return q
		}
		q := mq.NewStreamerQueue(streamerID, cfg.BufferCap)
		queues[streamerID] = q
		return q
	}

	// Dispatcher: dequeue from all StreamerQueues and route to collectors.
	go func() {
		for {
			queuesMu.Lock()
			for _, q := range queues {
				select {
				case msg := <-q.Ch:
					c := router.RouteForStreamer(msg.StreamerID)
					if c == nil {
						// No collectors — put message back and wait.
						// Back-pressure: the send channel will fill and
						// the upstream TCP read will block naturally.
						go func(m *mq.InFlightMessage, sq *mq.StreamerQueue) {
							time.Sleep(100 * time.Millisecond)
							sq.Ch <- m
						}(msg, q)
						continue
					}
					msg.CollectorID = c.ID
					tracker.Track(msg)
					f := &protocol.Frame{
						Type:  protocol.MsgTypeDeliver,
						MsgID: msg.MsgID,
						Body:  msg.Payload,
					}
					select {
					case c.Send <- f:
					default:
						// Collector send buffer full; redeliver via tracker timeout.
					}
				default:
				}
			}
			queuesMu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Millisecond):
			}
		}
	}()

	// Redelivery ticker: re-send timed-out in-flight messages.
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				for _, msg := range tracker.Expired(cfg.InFlightTimeout, cfg.MaxRetries) {
					c := router.RouteForStreamer(msg.StreamerID)
					if c == nil {
						continue
					}
					msg.CollectorID = c.ID
					f := &protocol.Frame{
						Type:  protocol.MsgTypeDeliver,
						MsgID: msg.MsgID,
						Body:  msg.Payload,
					}
					select {
					case c.Send <- f:
					default:
					}
				}
				for _, msg := range tracker.DroppedExpired(cfg.InFlightTimeout, cfg.MaxRetries) {
					logger.Warn("message dropped after max retries", "msg_id", msg.MsgID, "streamer", msg.StreamerID)
				}
			}
		}
	}()

	// WAL snapshot ticker.
	go func() {
		tick := time.NewTicker(cfg.SnapshotInterval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				queuesMu.Lock()
				checkpoint := &mq.Checkpoint{Streamers: make(map[string]*mq.StreamerOffset)}
				for id, q := range queues {
					checkpoint.Streamers[id] = &mq.StreamerOffset{LastSeq: q.LastSeq()}
				}
				queuesMu.Unlock()
				if err := wal.WriteCheckpoint(checkpoint); err != nil {
					logger.Error("checkpoint write failed", "err", err)
				}
			}
		}
	}()

	// Accept connections.
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				logger.Error("accept error", "err", err)
				continue
			}
		}
		go handleConn(ctx, conn, router, tracker, wal, cfg, getOrCreateQueue, logger)
	}
}

func handleConn(
	ctx context.Context,
	conn net.Conn,
	router *mq.Router,
	tracker *mq.InFlightTracker,
	wal *mq.WAL,
	cfg config.MQConfig,
	getOrCreateQueue func(string) *mq.StreamerQueue,
	logger *slog.Logger,
) {
	defer conn.Close()
	r := bufio.NewReader(conn)

	// Heartbeat dead-connection detection: reset after each successful read (T049).
	resetDeadline := func() {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	}
	resetDeadline()

	// First frame must be HELLO.
	f, err := protocol.ReadFrame(r)
	if err != nil {
		return
	}
	resetDeadline()
	if f.Type != protocol.MsgTypeHello {
		conn.Close()
		return
	}
	hello, err := protocol.UnmarshalHelloBody(f.Body)
	if err != nil {
		return
	}
	logger.Info("client connected", "node_type", hello.NodeType, "id", hello.ID)

	// ACK the HELLO.
	if err := protocol.WriteFrame(conn, protocol.NewACKFrame(0)); err != nil {
		return
	}

	switch hello.NodeType {
	case protocol.NodeTypeStreamer:
		handleStreamer(ctx, conn, r, hello.ID, getOrCreateQueue, wal, cfg, resetDeadline, logger)
	case protocol.NodeTypeCollector:
		handleCollector(ctx, conn, r, hello.ID, router, tracker, resetDeadline, logger)
	default:
		logger.Warn("unknown node type", "type", hello.NodeType)
	}
}

func handleStreamer(
	ctx context.Context,
	conn net.Conn,
	r *bufio.Reader,
	streamerID string,
	getOrCreateQueue func(string) *mq.StreamerQueue,
	wal *mq.WAL,
	cfg config.MQConfig,
	resetDeadline func(),
	logger *slog.Logger,
) {
	q := getOrCreateQueue(streamerID)

	// Heartbeat sender.
	go func() {
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if err := protocol.WriteFrame(conn, protocol.NewHeartbeatFrame()); err != nil {
					return
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		f, err := protocol.ReadFrame(r)
		if err != nil {
			return
		}
		resetDeadline()

		switch f.Type {
		case protocol.MsgTypePublish:
			pb, err := protocol.UnmarshalPublishBody(f.Body)
			if err != nil {
				protocol.WriteFrame(conn, protocol.NewNACKFrame(f.MsgID, protocol.NACKReasonInternalError))
				continue
			}
			msg := &mq.InFlightMessage{
				MsgID:      nextMsgID(),
				StreamerID: streamerID,
				SeqNum:     pb.SeqNum,
				SentAt:     time.Now(),
				Payload:    f.Body, // forward PublishBody bytes verbatim as DeliverBody
			}
			if err := q.Enqueue(msg); err != nil {
				// Buffer full → back-pressure: block further reads until space available.
				// The TCP receive window fills and the Streamer's write blocks (FR-004 edge).
				// We intentionally do NOT NACK here; we wait and retry.
				for {
					time.Sleep(10 * time.Millisecond)
					if err2 := q.Enqueue(msg); err2 == nil {
						break
					}
					select {
					case <-ctx.Done():
						return
					default:
					}
				}
			}
			// Append to WAL before ACKing so recovery is possible on crash.
			if walErr := wal.Append(streamerID, pb.SeqNum, f.Body); walErr != nil {
				logger.Error("WAL append failed", "err", walErr)
			}
			if err := protocol.WriteFrame(conn, protocol.NewACKFrame(f.MsgID)); err != nil {
				return
			}
		case protocol.MsgTypeHeartbeat:
			// Keepalive received — deadline already reset above.
		default:
			logger.Warn("unexpected frame from streamer", "type", fmt.Sprintf("0x%02x", f.Type))
		}
	}
}

func handleCollector(
	ctx context.Context,
	conn net.Conn,
	r *bufio.Reader,
	collectorID string,
	router *mq.Router,
	tracker *mq.InFlightTracker,
	resetDeadline func(),
	logger *slog.Logger,
) {
	sendCh := make(chan *protocol.Frame, 256)
	cc := &mq.CollectorConn{ID: collectorID, Conn: conn, Send: sendCh}
	router.Register(cc)
	defer func() {
		router.Deregister(collectorID)
		logger.Info("collector disconnected", "id", collectorID)
	}()

	// Write goroutine drains the send channel.
	go func() {
		for f := range sendCh {
			if err := protocol.WriteFrame(conn, f); err != nil {
				return
			}
		}
	}()

	// Heartbeat sender.
	go func() {
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				select {
				case sendCh <- protocol.NewHeartbeatFrame():
				default:
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		f, err := protocol.ReadFrame(r)
		if err != nil {
			return
		}
		resetDeadline()

		switch f.Type {
		case protocol.MsgTypeACK:
			ackID, err := protocol.ReadACKMsgID(f.Body)
			if err == nil {
				tracker.Ack(ackID)
			}
		case protocol.MsgTypeHeartbeat:
			// Keepalive received.
		default:
			logger.Warn("unexpected frame from collector", "type", fmt.Sprintf("0x%02x", f.Type))
		}
	}
}
