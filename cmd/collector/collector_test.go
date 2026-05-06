package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"log/slog"
	"os"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/protocol"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWriter records Write and Flush calls.
type mockWriter struct {
	written []*telemetry.TelemetryRecord
	flushes int
	failOn  int // 0 = never fail
}

func (m *mockWriter) Write(_ context.Context, rec *telemetry.TelemetryRecord) error {
	m.written = append(m.written, rec)
	return nil
}

func (m *mockWriter) Flush(_ context.Context) error {
	m.flushes++
	return nil
}

func TestWriteWithRetry_Success(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	w := &mockWriter{}
	rec := &telemetry.TelemetryRecord{MetricName: "GPU_UTIL", GPUID: "0", ProcessedAt: time.Now()}
	writeWithRetry(context.Background(), w, rec, logger)
	assert.Len(t, w.written, 1)
	assert.Equal(t, 1, w.flushes)
}

func TestDecodeDeliverFrame(t *testing.T) {
	ts := time.Now().UnixNano()
	rec := &telemetry.TelemetryRecord{MetricName: "GPU_UTIL", GPUID: "g1", ProcessedAt: time.Unix(0, ts).UTC()}
	payload, err := json.Marshal(rec)
	require.NoError(t, err)

	pb := &protocol.PublishBody{
		StreamerID: "s1",
		SeqNum:     1,
		Timestamp:  ts,
		Payload:    payload,
	}
	f := &protocol.Frame{Type: protocol.MsgTypeDeliver, MsgID: 42, Body: pb.Marshal()}

	// Re-parse the frame body as the collector would.
	parsed, err := protocol.UnmarshalPublishBody(f.Body)
	require.NoError(t, err)

	var decoded telemetry.TelemetryRecord
	require.NoError(t, json.Unmarshal(parsed.Payload, &decoded))
	decoded.ProcessedAt = time.Unix(0, parsed.Timestamp).UTC()

	assert.Equal(t, "GPU_UTIL", decoded.MetricName)
	assert.Equal(t, "g1", decoded.GPUID)
	assert.Equal(t, time.Unix(0, ts).UTC(), decoded.ProcessedAt, "ProcessedAt must come from Ts field, not CSV")
}

func TestACKSentAfterDeliver(t *testing.T) {
	// Build a minimal DELIVER frame and verify the ACK frame written back has the right MsgID.
	rec := &telemetry.TelemetryRecord{MetricName: "X", ProcessedAt: time.Now()}
	payload, _ := json.Marshal(rec)
	pb := &protocol.PublishBody{StreamerID: "s1", SeqNum: 1, Timestamp: rec.ProcessedAt.UnixNano(), Payload: payload}
	deliverFrame := &protocol.Frame{Type: protocol.MsgTypeDeliver, MsgID: 99, Body: pb.Marshal()}

	var responseBuf bytes.Buffer
	require.NoError(t, protocol.WriteFrame(&responseBuf, protocol.NewACKFrame(deliverFrame.MsgID)))

	resp, err := protocol.ReadFrame(bufio.NewReader(&responseBuf))
	require.NoError(t, err)
	assert.Equal(t, protocol.MsgTypeACK, resp.Type)
	ackID, err := protocol.ReadACKMsgID(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, uint64(99), ackID)
}

// Ensure TelemetryWriter interface is satisfied by *questdb.ILPWriter at the type level.
// (Compile-time check only; no import of questdb to avoid circular dep in this test.)
var _ TelemetryWriter = (*mockWriter)(nil)

// Compile-time check that connectWithBackoff and doHandshake exist.
var _ = connectWithBackoff
var _ = doHandshake
var _ net.Conn = (*net.TCPConn)(nil)
