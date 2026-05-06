package protocol_test

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteReadRoundtrip(t *testing.T) {
	original := &protocol.Frame{
		Type:  protocol.MsgTypePublish,
		MsgID: 42,
		Body:  []byte("hello world"),
	}

	var buf bytes.Buffer
	require.NoError(t, protocol.WriteFrame(&buf, original))

	got, err := protocol.ReadFrame(bufio.NewReader(&buf))
	require.NoError(t, err)

	assert.Equal(t, original.Type, got.Type)
	assert.Equal(t, original.MsgID, got.MsgID)
	assert.Equal(t, original.Body, got.Body)
}

func TestWriteReadEmptyBody(t *testing.T) {
	original := protocol.NewHeartbeatFrame()

	var buf bytes.Buffer
	require.NoError(t, protocol.WriteFrame(&buf, original))

	got, err := protocol.ReadFrame(bufio.NewReader(&buf))
	require.NoError(t, err)

	assert.Equal(t, protocol.MsgTypeHeartbeat, got.Type)
	assert.Equal(t, uint64(0), got.MsgID)
	assert.Empty(t, got.Body)
}

func TestHelloBodyRoundtrip(t *testing.T) {
	h := &protocol.HelloBody{NodeType: protocol.NodeTypeStreamer, ID: "streamer-1"}
	b := h.Marshal()
	got, err := protocol.UnmarshalHelloBody(b)
	require.NoError(t, err)
	assert.Equal(t, h.NodeType, got.NodeType)
	assert.Equal(t, h.ID, got.ID)
}

func TestPublishBodyRoundtrip(t *testing.T) {
	p := &protocol.PublishBody{
		StreamerID: "s1",
		SeqNum:     99,
		Timestamp:  1234567890,
		Payload:    []byte(`{"metric_name":"GPU_UTIL"}`),
	}
	b := p.Marshal()
	got, err := protocol.UnmarshalPublishBody(b)
	require.NoError(t, err)
	assert.Equal(t, p.StreamerID, got.StreamerID)
	assert.Equal(t, p.SeqNum, got.SeqNum)
	assert.Equal(t, p.Timestamp, got.Timestamp)
	assert.Equal(t, p.Payload, got.Payload)
}

func TestACKBodyRoundtrip(t *testing.T) {
	a := &protocol.ACKBody{AckMsgID: 77}
	b := a.Marshal()
	got, err := protocol.UnmarshalACKBody(b)
	require.NoError(t, err)
	assert.Equal(t, a.AckMsgID, got.AckMsgID)
}

func TestNACKBodyRoundtrip(t *testing.T) {
	n := &protocol.NACKBody{NackMsgID: 55, ReasonCode: protocol.NACKReasonBufferFull}
	b := n.Marshal()
	got, err := protocol.UnmarshalNACKBody(b)
	require.NoError(t, err)
	assert.Equal(t, n.NackMsgID, got.NackMsgID)
	assert.Equal(t, n.ReasonCode, got.ReasonCode)
}

func TestMultipleFramesInBuffer(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 5; i++ {
		f := &protocol.Frame{Type: protocol.MsgTypeACK, MsgID: uint64(i), Body: []byte{byte(i)}}
		require.NoError(t, protocol.WriteFrame(&buf, f))
	}
	r := bufio.NewReader(&buf)
	for i := 0; i < 5; i++ {
		f, err := protocol.ReadFrame(r)
		require.NoError(t, err)
		assert.Equal(t, uint64(i), f.MsgID)
	}
}
