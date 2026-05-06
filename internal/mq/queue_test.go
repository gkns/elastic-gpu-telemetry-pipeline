package mq_test

import (
	"testing"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/mq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeMsg(msgID, seqNum uint64, streamerID string) *mq.InFlightMessage {
	return &mq.InFlightMessage{
		MsgID:      msgID,
		StreamerID: streamerID,
		SeqNum:     seqNum,
		SentAt:     time.Now(),
		Payload:    []byte(`{}`),
	}
}

func TestStreamerQueue_InOrder(t *testing.T) {
	q := mq.NewStreamerQueue("s1", 100)
	for i := uint64(1); i <= 5; i++ {
		err := q.Enqueue(makeMsg(i, i, "s1"))
		require.NoError(t, err)
	}
	assert.Equal(t, uint64(5), q.LastSeq())
}

func TestStreamerQueue_OutOfOrder(t *testing.T) {
	q := mq.NewStreamerQueue("s1", 100)
	require.NoError(t, q.Enqueue(makeMsg(1, 5, "s1")))
	err := q.Enqueue(makeMsg(2, 3, "s1")) // 3 < 5 → out of order
	assert.Error(t, err)
	assert.Equal(t, uint64(5), q.LastSeq())
}

func TestStreamerQueue_DuplicateSeq(t *testing.T) {
	q := mq.NewStreamerQueue("s1", 100)
	require.NoError(t, q.Enqueue(makeMsg(1, 1, "s1")))
	err := q.Enqueue(makeMsg(2, 1, "s1")) // same seq → reject
	assert.Error(t, err)
}

func TestStreamerQueue_BufferFull(t *testing.T) {
	q := mq.NewStreamerQueue("s1", 2)
	require.NoError(t, q.Enqueue(makeMsg(1, 1, "s1")))
	require.NoError(t, q.Enqueue(makeMsg(2, 2, "s1")))
	err := q.Enqueue(makeMsg(3, 3, "s1"))
	assert.Error(t, err)
}
