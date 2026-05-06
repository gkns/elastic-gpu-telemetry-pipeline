package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

type MsgType uint8

const (
	MsgTypePublish   MsgType = 0x01
	MsgTypeACK       MsgType = 0x02
	MsgTypeNACK      MsgType = 0x03
	MsgTypeHello     MsgType = 0x04
	MsgTypeDeliver   MsgType = 0x05
	MsgTypeHeartbeat MsgType = 0x06
)

type NodeType uint8

const (
	NodeTypeStreamer  NodeType = 0x01
	NodeTypeCollector NodeType = 0x02
)

// NACKReason codes
const (
	NACKReasonBufferFull    uint8 = 0x01
	NACKReasonOutOfOrder    uint8 = 0x02
	NACKReasonInternalError uint8 = 0x03
)

// Frame header: Type(1B) + Length(4B) + MsgID(8B) = 13 bytes
const HeaderSize = 13

type Frame struct {
	Type  MsgType
	MsgID uint64
	Body  []byte
}

// HelloBody is the first frame sent by any client after TCP connect.
type HelloBody struct {
	NodeType NodeType
	ID       string
}

func (h *HelloBody) Marshal() []byte {
	idBytes := []byte(h.ID)
	buf := make([]byte, 1+2+len(idBytes))
	buf[0] = byte(h.NodeType)
	binary.BigEndian.PutUint16(buf[1:3], uint16(len(idBytes)))
	copy(buf[3:], idBytes)
	return buf
}

func UnmarshalHelloBody(b []byte) (*HelloBody, error) {
	if len(b) < 3 {
		return nil, fmt.Errorf("hello body too short: %d", len(b))
	}
	h := &HelloBody{NodeType: NodeType(b[0])}
	idLen := binary.BigEndian.Uint16(b[1:3])
	if len(b) < 3+int(idLen) {
		return nil, fmt.Errorf("hello body truncated: need %d got %d", 3+idLen, len(b))
	}
	h.ID = string(b[3 : 3+idLen])
	return h, nil
}

// PublishBody is sent by Streamer → MQ (MsgTypePublish) and forwarded
// verbatim by MQ → Collector (MsgTypeDeliver).
type PublishBody struct {
	StreamerID string
	SeqNum     uint64
	Timestamp  int64  // Unix nanoseconds
	Payload    []byte // JSON-encoded TelemetryRecord
}

func (p *PublishBody) Marshal() []byte {
	idBytes := []byte(p.StreamerID)
	size := 2 + len(idBytes) + 8 + 8 + 4 + len(p.Payload)
	buf := make([]byte, size)
	off := 0
	binary.BigEndian.PutUint16(buf[off:], uint16(len(idBytes)))
	off += 2
	copy(buf[off:], idBytes)
	off += len(idBytes)
	binary.BigEndian.PutUint64(buf[off:], p.SeqNum)
	off += 8
	binary.BigEndian.PutUint64(buf[off:], uint64(p.Timestamp))
	off += 8
	binary.BigEndian.PutUint32(buf[off:], uint32(len(p.Payload)))
	off += 4
	copy(buf[off:], p.Payload)
	return buf
}

func UnmarshalPublishBody(b []byte) (*PublishBody, error) {
	if len(b) < 2 {
		return nil, fmt.Errorf("publish body too short")
	}
	p := &PublishBody{}
	off := 0
	idLen := int(binary.BigEndian.Uint16(b[off:]))
	off += 2
	if len(b) < off+idLen+8+8+4 {
		return nil, fmt.Errorf("publish body truncated")
	}
	p.StreamerID = string(b[off : off+idLen])
	off += idLen
	p.SeqNum = binary.BigEndian.Uint64(b[off:])
	off += 8
	p.Timestamp = int64(binary.BigEndian.Uint64(b[off:]))
	off += 8
	payLen := int(binary.BigEndian.Uint32(b[off:]))
	off += 4
	if len(b) < off+payLen {
		return nil, fmt.Errorf("publish body payload truncated")
	}
	p.Payload = make([]byte, payLen)
	copy(p.Payload, b[off:off+payLen])
	return p, nil
}

// ACKBody acknowledges a frame by MsgID.
type ACKBody struct {
	AckMsgID uint64
}

func (a *ACKBody) Marshal() []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, a.AckMsgID)
	return b
}

func UnmarshalACKBody(b []byte) (*ACKBody, error) {
	if len(b) < 8 {
		return nil, fmt.Errorf("ack body too short")
	}
	return &ACKBody{AckMsgID: binary.BigEndian.Uint64(b)}, nil
}

// NACKBody rejects a frame with a reason code.
type NACKBody struct {
	NackMsgID  uint64
	ReasonCode uint8
}

func (n *NACKBody) Marshal() []byte {
	b := make([]byte, 9)
	binary.BigEndian.PutUint64(b, n.NackMsgID)
	b[8] = n.ReasonCode
	return b
}

func UnmarshalNACKBody(b []byte) (*NACKBody, error) {
	if len(b) < 9 {
		return nil, fmt.Errorf("nack body too short")
	}
	return &NACKBody{
		NackMsgID:  binary.BigEndian.Uint64(b),
		ReasonCode: b[8],
	}, nil
}

// NewACKFrame builds an ACK frame for the given MsgID.
func NewACKFrame(ackMsgID uint64) *Frame {
	body := &ACKBody{AckMsgID: ackMsgID}
	return &Frame{Type: MsgTypeACK, MsgID: 0, Body: body.Marshal()}
}

// NewNACKFrame builds a NACK frame for the given MsgID and reason.
func NewNACKFrame(nackMsgID uint64, reason uint8) *Frame {
	body := &NACKBody{NackMsgID: nackMsgID, ReasonCode: reason}
	return &Frame{Type: MsgTypeNACK, MsgID: 0, Body: body.Marshal()}
}

// NewHeartbeatFrame builds a heartbeat frame.
func NewHeartbeatFrame() *Frame {
	return &Frame{Type: MsgTypeHeartbeat, MsgID: 0, Body: nil}
}

// ReadACKMsgID extracts AckMsgID from an ACK frame body without full unmarshal.
func ReadACKMsgID(body []byte) (uint64, error) {
	a, err := UnmarshalACKBody(body)
	if err != nil {
		return 0, err
	}
	return a.AckMsgID, nil
}

// Encode serialises a Frame to w. Exported for use by codec_test.
func (f *Frame) Encode(w io.Writer) error {
	hdr := make([]byte, HeaderSize)
	hdr[0] = byte(f.Type)
	binary.BigEndian.PutUint32(hdr[1:5], uint32(len(f.Body)))
	binary.BigEndian.PutUint64(hdr[5:13], f.MsgID)
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	if len(f.Body) > 0 {
		_, err := w.Write(f.Body)
		return err
	}
	return nil
}
