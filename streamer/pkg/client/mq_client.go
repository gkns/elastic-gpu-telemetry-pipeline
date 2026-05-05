package client

import (
	"fmt"
	"net"
	"sync"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/protocol"
)

type MQClient struct {
	conn       net.Conn
	streamerID [16]byte
	mu         sync.Mutex
}

func NewMQClient(addr string, streamerID [16]byte) (*MQClient, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	c := &MQClient{
		conn:       conn,
		streamerID: streamerID,
	}

	if err := c.handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	return c, nil
}

func (c *MQClient) handshake() error {
	msg := &protocol.Message{
		Type:       protocol.TypeStreamerHandshake,
		StreamerID: c.streamerID,
	}

	if err := msg.Encode(c.conn); err != nil {
		return err
	}

	ack, err := protocol.Decode(c.conn)
	if err != nil {
		return err
	}

	if ack.Type != protocol.TypeACK {
		return fmt.Errorf("handshake failed: received %v", ack.Type)
	}

	return nil
}

func (c *MQClient) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	msg := &protocol.Message{
		Type:       protocol.TypeTelemetryData,
		StreamerID: c.streamerID,
		Payload:    data,
	}

	if err := msg.Encode(c.conn); err != nil {
		return err
	}

	ack, err := protocol.Decode(c.conn)
	if err != nil {
		return err
	}

	if ack.Type == protocol.TypeNACK {
		return fmt.Errorf("message rejected by MQ (back-pressure)")
	}

	if ack.Type != protocol.TypeACK {
		return fmt.Errorf("unexpected response: %v", ack.Type)
	}

	return nil
}

func (c *MQClient) Close() error {
	return c.conn.Close()
}
