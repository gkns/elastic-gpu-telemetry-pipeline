package mq

import (
	"sync"
	"time"
)

// InFlightMessage tracks a message dispatched to a Collector but not yet ACKed.
type InFlightMessage struct {
	MsgID       uint64
	CollectorID string
	StreamerID  string
	SeqNum      uint64
	SentAt      time.Time
	Retries     int
	Payload     []byte // original PublishBody bytes for redelivery
}

// InFlightTracker is a thread-safe registry of in-flight messages.
type InFlightTracker struct {
	mu       sync.Mutex
	messages map[uint64]*InFlightMessage
}

func NewInFlightTracker() *InFlightTracker {
	return &InFlightTracker{messages: make(map[uint64]*InFlightMessage)}
}

// Track registers a message as in-flight.
func (t *InFlightTracker) Track(msg *InFlightMessage) {
	t.mu.Lock()
	t.messages[msg.MsgID] = msg
	t.mu.Unlock()
}

// Ack removes a message from the in-flight registry.
func (t *InFlightTracker) Ack(msgID uint64) {
	t.mu.Lock()
	delete(t.messages, msgID)
	t.mu.Unlock()
}

// Expired returns all messages older than timeout that have Retries < maxRetries,
// incrementing each entry's Retries and updating SentAt before returning.
func (t *InFlightTracker) Expired(timeout time.Duration, maxRetries int) []*InFlightMessage {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	var expired []*InFlightMessage
	for _, msg := range t.messages {
		if now.Sub(msg.SentAt) >= timeout {
			if msg.Retries >= maxRetries {
				// Caller is responsible for removing via Ack after logging drop
				continue
			}
			msg.Retries++
			msg.SentAt = now
			expired = append(expired, msg)
		}
	}
	return expired
}

// DroppedExpired returns messages that have exceeded maxRetries and removes them.
func (t *InFlightTracker) DroppedExpired(timeout time.Duration, maxRetries int) []*InFlightMessage {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	var dropped []*InFlightMessage
	for id, msg := range t.messages {
		if now.Sub(msg.SentAt) >= timeout && msg.Retries >= maxRetries {
			dropped = append(dropped, msg)
			delete(t.messages, id)
		}
	}
	return dropped
}

// ForCollector returns all in-flight messages assigned to the given collector.
func (t *InFlightTracker) ForCollector(collectorID string) []*InFlightMessage {
	t.mu.Lock()
	defer t.mu.Unlock()
	var msgs []*InFlightMessage
	for _, m := range t.messages {
		if m.CollectorID == collectorID {
			msgs = append(msgs, m)
		}
	}
	return msgs
}

// Size returns the number of tracked messages.
func (t *InFlightTracker) Size() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.messages)
}
