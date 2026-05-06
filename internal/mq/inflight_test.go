package mq_test

import (
	"testing"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/mq"
	"github.com/stretchr/testify/assert"
)

func TestInFlightTracker_AckRemovesEntry(t *testing.T) {
	tr := mq.NewInFlightTracker()
	msg := &mq.InFlightMessage{MsgID: 1, StreamerID: "s1", SeqNum: 1, SentAt: time.Now()}
	tr.Track(msg)
	assert.Equal(t, 1, tr.Size())
	tr.Ack(1)
	assert.Equal(t, 0, tr.Size())
}

func TestInFlightTracker_ExpiredReturnsTimedOut(t *testing.T) {
	tr := mq.NewInFlightTracker()
	old := &mq.InFlightMessage{MsgID: 1, StreamerID: "s1", SeqNum: 1, SentAt: time.Now().Add(-10 * time.Second)}
	fresh := &mq.InFlightMessage{MsgID: 2, StreamerID: "s1", SeqNum: 2, SentAt: time.Now()}
	tr.Track(old)
	tr.Track(fresh)

	expired := tr.Expired(5*time.Second, 3)
	assert.Len(t, expired, 1)
	assert.Equal(t, uint64(1), expired[0].MsgID)
	assert.Equal(t, 1, expired[0].Retries) // incremented
}

func TestInFlightTracker_ExpiredIncrementsRetries(t *testing.T) {
	tr := mq.NewInFlightTracker()
	msg := &mq.InFlightMessage{MsgID: 1, SeqNum: 1, SentAt: time.Now().Add(-10 * time.Second), Retries: 2}
	tr.Track(msg)
	expired := tr.Expired(5*time.Second, 3)
	assert.Len(t, expired, 1)
	assert.Equal(t, 3, expired[0].Retries)
}

func TestInFlightTracker_ExpiredExcludesMaxRetries(t *testing.T) {
	tr := mq.NewInFlightTracker()
	msg := &mq.InFlightMessage{MsgID: 1, SeqNum: 1, SentAt: time.Now().Add(-10 * time.Second), Retries: 3}
	tr.Track(msg)
	expired := tr.Expired(5*time.Second, 3)
	assert.Empty(t, expired)
}

func TestInFlightTracker_DroppedExpiredRemoves(t *testing.T) {
	tr := mq.NewInFlightTracker()
	msg := &mq.InFlightMessage{MsgID: 1, SeqNum: 1, SentAt: time.Now().Add(-10 * time.Second), Retries: 3}
	tr.Track(msg)
	dropped := tr.DroppedExpired(5*time.Second, 3)
	assert.Len(t, dropped, 1)
	assert.Equal(t, 0, tr.Size())
}
