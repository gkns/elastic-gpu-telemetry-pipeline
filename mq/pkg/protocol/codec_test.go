package protocol

import (
	"bytes"
	"testing"
)

func TestCodec(t *testing.T) {
	msg := &Message{
		Type:       TypeTelemetryData,
		MessageID:  [16]byte{0x01, 0x02},
		StreamerID: [16]byte{0x0A, 0x0B},
		Payload:    []byte("hello world"),
	}

	buf := new(bytes.Buffer)
	if err := msg.Encode(buf); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(buf)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("expected type %v, got %v", msg.Type, decoded.Type)
	}

	if !bytes.Equal(decoded.Payload, msg.Payload) {
		t.Errorf("expected payload %s, got %s", msg.Payload, decoded.Payload)
	}
}
