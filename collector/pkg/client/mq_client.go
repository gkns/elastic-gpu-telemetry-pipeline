package client

import (
	"fmt"
	"net"
	"sync"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/protocol"
)

type MQClient struct {
	conn        net.Conn
	collectorID [16]byte
	mu          sync.Mutex
}

func NewMQClient(addr string, collectorID [16]byte) (*MQClient, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	c := &MQClient{
		conn:        conn,
		collectorID: collectorID,
	}

	if err := c.handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	return c, nil
}

func (c *MQClient) handshake() error {
	msg := &protocol.Message{
		Type:       protocol.TypeCollectorHandshake,
		StreamerID: c.collectorID, // Use StreamerID field as NodeID
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

func (c *MQClient) Receive() (*protocol.Message, error) {
	return protocol.Decode(c.conn)
}

func (c *MQClient) ACK(msgID [16]byte, streamerID [16]byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ack := &protocol.Message{
		Type:       protocol.TypeACK,
		MessageID:  msgID,
		StreamerID: streamerID,
	}

	return ack.Encode(c.conn)
}

func (c *MQClient) Close() error {
	return c.conn.Close()
}
