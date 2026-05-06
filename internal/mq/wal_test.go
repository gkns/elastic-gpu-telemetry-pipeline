package mq_test

import (
	"os"
	"testing"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/mq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWAL_AppendAndReplay(t *testing.T) {
	dir := t.TempDir()
	w, err := mq.NewWAL(dir)
	require.NoError(t, err)

	payload := []byte(`{"metric_name":"GPU_UTIL"}`)
	require.NoError(t, w.Append("streamer-1", 1, payload))
	require.NoError(t, w.Append("streamer-1", 2, payload))
	require.NoError(t, w.Append("streamer-2", 1, payload))

	cp, err := w.ReadCheckpoint()
	require.NoError(t, err)

	msgs, err := w.ReplayFrom(cp)
	require.NoError(t, err)
	assert.Len(t, msgs, 3)
}

func TestWAL_Checkpoint_WriteRead(t *testing.T) {
	dir := t.TempDir()
	w, err := mq.NewWAL(dir)
	require.NoError(t, err)

	cp := &mq.Checkpoint{
		Streamers: map[string]*mq.StreamerOffset{
			"s1": {LastSeq: 42, WALOffset: 1024},
		},
	}
	require.NoError(t, w.WriteCheckpoint(cp))

	loaded, err := w.ReadCheckpoint()
	require.NoError(t, err)
	assert.Equal(t, uint64(42), loaded.Streamers["s1"].LastSeq)
	assert.Equal(t, int64(1024), loaded.Streamers["s1"].WALOffset)
}

func TestWAL_ReadCheckpoint_Missing(t *testing.T) {
	dir := t.TempDir()
	w, err := mq.NewWAL(dir)
	require.NoError(t, err)

	cp, err := w.ReadCheckpoint()
	require.NoError(t, err)
	assert.NotNil(t, cp)
	assert.Empty(t, cp.Streamers)
}

func TestWAL_CreateDir(t *testing.T) {
	dir := t.TempDir() + "/nested/wal"
	w, err := mq.NewWAL(dir)
	require.NoError(t, err)
	require.NotNil(t, w)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
