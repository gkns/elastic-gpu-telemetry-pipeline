package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	HeaderSize = 37 // 4 + 1 + 16 + 16
)

type MessageType uint8

const (
	TypeStreamerHandshake  MessageType = 0x01
	TypeCollectorHandshake MessageType = 0x02
	TypeTelemetryData      MessageType = 0x03
	TypeACK                MessageType = 0x04
	TypeNACK               MessageType = 0x05
)

type Message struct {
	TotalLength uint32
	Type        MessageType
	MessageID   [16]byte
	StreamerID  [16]byte
	Payload     []byte
}

func (m *Message) Encode(w io.Writer) error {
	m.TotalLength = uint32(HeaderSize + len(m.Payload))
	if err := binary.Write(w, binary.BigEndian, m.TotalLength); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, m.Type); err != nil {
		return err
	}
	if _, err := w.Write(m.MessageID[:]); err != nil {
		return err
	}
	if _, err := w.Write(m.StreamerID[:]); err != nil {
		return err
	}
	if _, err := w.Write(m.Payload); err != nil {
		return err
	}
	return nil
}

func Decode(r io.Reader) (*Message, error) {
	var totalLength uint32
	if err := binary.Read(r, binary.BigEndian, &totalLength); err != nil {
		return nil, err
	}

	if totalLength < HeaderSize {
		return nil, errors.New("invalid message length")
	}

	var msgType MessageType
	if err := binary.Read(r, binary.BigEndian, &msgType); err != nil {
		return nil, err
	}

	var msgID [16]byte
	if _, err := io.ReadFull(r, msgID[:]); err != nil {
		return nil, err
	}

	var streamerID [16]byte
	if _, err := io.ReadFull(r, streamerID[:]); err != nil {
		return nil, err
	}

	payloadLength := totalLength - HeaderSize
	payload := make([]byte, payloadLength)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	return &Message{
		TotalLength: totalLength,
		Type:        msgType,
		MessageID:   msgID,
		StreamerID:  streamerID,
		Payload:     payload,
	}, nil
}

func (t MessageType) String() string {
	switch t {
	case TypeStreamerHandshake:
		return "StreamerHandshake"
	case TypeCollectorHandshake:
		return "CollectorHandshake"
	case TypeTelemetryData:
		return "TelemetryData"
	case TypeACK:
		return "ACK"
	case TypeNACK:
		return "NACK"
	default:
		return fmt.Sprintf("Unknown(0x%02x)", uint8(t))
	}
}
