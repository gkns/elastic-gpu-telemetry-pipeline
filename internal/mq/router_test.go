package mq_test

import (
	"testing"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/mq"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCollector(id string) *mq.CollectorConn {
	return &mq.CollectorConn{
		ID:   id,
		Send: make(chan *protocol.Frame, 64),
	}
}

func TestRouter_StickyRouting(t *testing.T) {
	r := &mq.Router{}
	r.Register(newCollector("c1"))
	r.Register(newCollector("c2"))

	first := r.RouteForStreamer("streamer-a")
	require.NotNil(t, first)
	for i := 0; i < 20; i++ {
		got := r.RouteForStreamer("streamer-a")
		assert.Equal(t, first.ID, got.ID)
	}
}

func TestRouter_DifferentStreamersCanMapToDifferentCollectors(t *testing.T) {
	r := &mq.Router{}
	for i := 0; i < 10; i++ {
		r.Register(newCollector(string(rune('a' + i))))
	}
	seen := map[string]struct{}{}
	streamers := []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9", "s10"}
	for _, s := range streamers {
		c := r.RouteForStreamer(s)
		require.NotNil(t, c)
		seen[c.ID] = struct{}{}
	}
	assert.Greater(t, len(seen), 1)
}

func TestRouter_DeregisterRehash(t *testing.T) {
	r := &mq.Router{}
	r.Register(newCollector("c1"))
	r.Register(newCollector("c2"))

	r.Deregister("c1")
	assert.Equal(t, 1, r.Len())

	for _, s := range []string{"s1", "s2", "s3"} {
		c := r.RouteForStreamer(s)
		require.NotNil(t, c)
		assert.Equal(t, "c2", c.ID)
	}
}

func TestRouter_NoCollectors(t *testing.T) {
	r := &mq.Router{}
	assert.Nil(t, r.RouteForStreamer("s1"))
}

func TestRouter_CollectorIDs(t *testing.T) {
	r := &mq.Router{}
	r.Register(newCollector("c1"))
	r.Register(newCollector("c2"))
	ids := r.CollectorIDs()
	assert.ElementsMatch(t, []string{"c1", "c2"}, ids)
}
