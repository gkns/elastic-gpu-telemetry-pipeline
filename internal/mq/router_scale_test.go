package mq_test

import (
	"fmt"
	"testing"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/mq"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouter_ScaleUp(t *testing.T) {
	r := &mq.Router{}
	for i := 0; i < 10; i++ {
		r.Register(&mq.CollectorConn{ID: fmt.Sprintf("c%d", i), Send: make(chan *protocol.Frame, 64)})
	}

	// Map each of 10 streamers to a collector; routing must be deterministic.
	initial := map[string]string{}
	streamers := make([]string, 10)
	for i := range streamers {
		streamers[i] = fmt.Sprintf("streamer-%d", i)
		c := r.RouteForStreamer(streamers[i])
		require.NotNil(t, c)
		initial[streamers[i]] = c.ID
	}

	// Route 100 messages for each streamer — same collector every time.
	for _, s := range streamers {
		for j := 0; j < 100; j++ {
			c := r.RouteForStreamer(s)
			require.NotNil(t, c)
			assert.Equal(t, initial[s], c.ID, "sticky routing violated for %s", s)
		}
	}
}

func TestRouter_ScaleDown(t *testing.T) {
	r := &mq.Router{}
	for i := 0; i < 5; i++ {
		r.Register(&mq.CollectorConn{ID: fmt.Sprintf("c%d", i), Send: make(chan *protocol.Frame, 64)})
	}

	// Deregister collectors one by one; routing must never panic and must remain valid.
	for i := 0; i < 4; i++ {
		r.Deregister(fmt.Sprintf("c%d", i))
		assert.Equal(t, 4-i, r.Len())
		for _, s := range []string{"s1", "s2", "s3"} {
			c := r.RouteForStreamer(s)
			require.NotNil(t, c, "nil collector after deregistering c%d", i)
		}
	}
}

func TestRouter_RebalanceAfterDeregister(t *testing.T) {
	r := &mq.Router{}
	r.Register(&mq.CollectorConn{ID: "c1", Send: make(chan *protocol.Frame, 64)})
	r.Register(&mq.CollectorConn{ID: "c2", Send: make(chan *protocol.Frame, 64)})

	before := r.RouteForStreamer("s-special")
	require.NotNil(t, before)

	// Deregister whichever collector s-special maps to.
	r.Deregister(before.ID)

	// After deregister, s-special must still route to the remaining collector.
	after := r.RouteForStreamer("s-special")
	require.NotNil(t, after)
	assert.NotEqual(t, before.ID, after.ID, "must route to a different collector after deregister")
}

func TestRouter_NoPanic_AllDeregistered(t *testing.T) {
	r := &mq.Router{}
	r.Register(&mq.CollectorConn{ID: "c1", Send: make(chan *protocol.Frame, 64)})
	r.Deregister("c1")
	assert.Nil(t, r.RouteForStreamer("s1"), "must return nil when no collectors available")
}
