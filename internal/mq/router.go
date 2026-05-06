package mq

import (
	"hash/fnv"
	"net"
	"sync"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/protocol"
)

// CollectorConn represents a live collector connection managed by the Router.
type CollectorConn struct {
	ID   string
	Conn net.Conn
	Send chan *protocol.Frame // outbound frame queue for this collector
}

// Router manages active collectors and performs sticky FNV-1a hash routing
// by streamer ID so all messages from one streamer go to the same collector,
// preserving per-streamer ordering (FR-009).
type Router struct {
	mu         sync.RWMutex
	collectors []*CollectorConn
}

// Register adds a new collector to the routing table.
func (r *Router) Register(c *CollectorConn) {
	r.mu.Lock()
	r.collectors = append(r.collectors, c)
	r.mu.Unlock()
}

// Deregister removes a collector by ID and returns the slice of active collectors
// that remain, so the caller can rehash any in-flight messages.
func (r *Router) Deregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.collectors[:0]
	for _, c := range r.collectors {
		if c.ID != id {
			out = append(out, c)
		}
	}
	r.collectors = out
}

// RouteForStreamer returns the CollectorConn assigned to the given streamer using
// FNV-1a hash mod len(collectors). Returns nil if no collectors are registered.
func (r *Router) RouteForStreamer(streamerID string) *CollectorConn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.collectors) == 0 {
		return nil
	}
	h := fnv.New32a()
	h.Write([]byte(streamerID))
	idx := int(h.Sum32()) % len(r.collectors)
	return r.collectors[idx]
}

// CollectorIDs returns a snapshot of currently registered collector IDs.
func (r *Router) CollectorIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, len(r.collectors))
	for i, c := range r.collectors {
		ids[i] = c.ID
	}
	return ids
}

// Len returns the number of registered collectors.
func (r *Router) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.collectors)
}
