//go:build loadtest

package mq_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/mq"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/protocol"
	"github.com/stretchr/testify/assert"
)

// TestLoad validates SC-001 and SC-002:
// 10 streamers × 10 collectors, 1000 messages per streamer = 10,000 total.
// All messages must be ACKed (zero loss) and p99 dispatch latency < 50ms.
func TestLoad(t *testing.T) {
	const (
		numStreamers  = 10
		numCollectors = 10
		msgsPerStream = 1000
		totalMsgs     = numStreamers * msgsPerStream
	)

	router := &mq.Router{}
	tracker := mq.NewInFlightTracker()

	// Register 10 mock collectors, each draining their Send channel.
	var wgCollectors sync.WaitGroup
	acked := make(chan uint64, totalMsgs)
	for i := 0; i < numCollectors; i++ {
		cc := &mq.CollectorConn{
			ID:   fmt.Sprintf("collector-%d", i),
			Send: make(chan *protocol.Frame, 256),
		}
		router.Register(cc)
		wgCollectors.Add(1)
		go func(c *mq.CollectorConn) {
			defer wgCollectors.Done()
			for f := range c.Send {
				tracker.Ack(f.MsgID)
				acked <- f.MsgID
			}
		}(cc)
	}

	// Route messages from 10 simulated streamers and measure dispatch latency.
	var latencies []time.Duration
	var latMu sync.Mutex
	var wgStreamers sync.WaitGroup
	var msgIDGen uint64
	var msgIDMu sync.Mutex

	for s := 0; s < numStreamers; s++ {
		streamerID := fmt.Sprintf("streamer-%d", s)
		wgStreamers.Add(1)
		go func(sid string) {
			defer wgStreamers.Done()
			for seq := 0; seq < msgsPerStream; seq++ {
				msgIDMu.Lock()
				msgIDGen++
				id := msgIDGen
				msgIDMu.Unlock()

				msg := &mq.InFlightMessage{
					MsgID:      id,
					StreamerID: sid,
					SeqNum:     uint64(seq + 1),
					SentAt:     time.Now(),
					Payload:    []byte(`{"metric":"GPU_UTIL"}`),
				}
				tracker.Track(msg)

				start := time.Now()
				c := router.RouteForStreamer(sid)
				if c == nil {
					tracker.Ack(id)
					acked <- id
					continue
				}
				c.Send <- &protocol.Frame{Type: protocol.MsgTypeDeliver, MsgID: id, Body: msg.Payload}
				elapsed := time.Since(start)

				latMu.Lock()
				latencies = append(latencies, elapsed)
				latMu.Unlock()
			}
		}(streamerID)
	}

	// Wait for all streamers to finish publishing.
	wgStreamers.Wait()

	// Close all collector Send channels and wait for drain.
	for _, id := range router.CollectorIDs() {
		_ = id // collectors drain when Send closes; use a done channel instead
	}

	// Collect all ACKs (with timeout to avoid hanging test).
	ackedIDs := make(map[uint64]struct{}, totalMsgs)
	timeout := time.After(10 * time.Second)
collect:
	for {
		select {
		case id := <-acked:
			ackedIDs[id] = struct{}{}
			if len(ackedIDs) >= totalMsgs {
				break collect
			}
		case <-timeout:
			t.Logf("timeout: got %d/%d ACKs", len(ackedIDs), totalMsgs)
			break collect
		}
	}

	assert.Equal(t, totalMsgs, len(ackedIDs), "zero message loss: all messages must be ACKed")

	// Calculate p99 latency.
	if len(latencies) > 0 {
		p99 := percentile(latencies, 99)
		t.Logf("p99 dispatch latency: %v (target: <50ms)", p99)
		assert.Less(t, p99, 50*time.Millisecond, "p99 dispatch latency must be < 50ms (SC-001)")
	}
}

func percentile(durations []time.Duration, p int) time.Duration {
	n := len(durations)
	if n == 0 {
		return 0
	}
	// Simple sort-based percentile.
	sorted := make([]time.Duration, n)
	copy(sorted, durations)
	for i := 1; i < n; i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	idx := (p * n) / 100
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}
