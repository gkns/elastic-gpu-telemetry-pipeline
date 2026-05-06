package mq

import (
	"fmt"
	"sync/atomic"
)

// StreamerQueue is a per-streamer FIFO buffer between ingestion and routing.
type StreamerQueue struct {
	StreamerID string
	Ch         chan *InFlightMessage
	lastSeq    atomic.Uint64
}

func NewStreamerQueue(id string, cap int) *StreamerQueue {
	return &StreamerQueue{
		StreamerID: id,
		Ch:         make(chan *InFlightMessage, cap),
	}
}

// Enqueue validates SeqNum ordering and adds msg to the channel.
// Returns an error if SeqNum is not strictly increasing or the buffer is full.
func (q *StreamerQueue) Enqueue(msg *InFlightMessage) error {
	prev := q.lastSeq.Load()
	if msg.SeqNum <= prev {
		return fmt.Errorf("out-of-order: got %d want >%d", msg.SeqNum, prev)
	}
	select {
	case q.Ch <- msg:
		q.lastSeq.Store(msg.SeqNum)
		return nil
	default:
		return fmt.Errorf("buffer full (cap %d)", cap(q.Ch))
	}
}

// LastSeq returns the last successfully enqueued sequence number.
func (q *StreamerQueue) LastSeq() uint64 {
	return q.lastSeq.Load()
}
