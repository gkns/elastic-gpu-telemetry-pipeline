package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/protocol"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/queue"
)

type MQHandler struct {
	buffer          *queue.Buffer
	balancer        *queue.Balancer
	streamerLocks   map[string]*sync.Mutex
	unackedMessages map[string]*protocol.Message
	mu              sync.Mutex
}

func NewMQHandler(buffer *queue.Buffer) *MQHandler {
	h := &MQHandler{
		buffer:          buffer,
		balancer:        queue.NewBalancer(),
		streamerLocks:   make(map[string]*sync.Mutex),
		unackedMessages: make(map[string]*protocol.Message),
	}
	go h.dispatchLoop()
	go h.timeoutLoop()
	return h
}

func (h *MQHandler) dispatchLoop() {
	for {
		msg := h.buffer.Pop()
		go h.dispatchWithOrder(msg)
	}
}

func (h *MQHandler) timeoutLoop() {
	for {
		time.Sleep(10 * time.Second)
		h.mu.Lock()
		for id, msg := range h.unackedMessages {
			fmt.Printf("Message %s timed out, re-queuing\n", id)
			h.buffer.Push(msg)
			delete(h.unackedMessages, id)
		}
		h.mu.Unlock()
	}
}

func (h *MQHandler) dispatchWithOrder(msg *protocol.Message) {
	sid := fmt.Sprintf("%x", msg.StreamerID)

	h.mu.Lock()
	lock, ok := h.streamerLocks[sid]
	if !ok {
		lock = &sync.Mutex{}
		h.streamerLocks[sid] = lock
	}
	h.mu.Unlock()

	lock.Lock()
	defer lock.Unlock()

	h.dispatch(msg)
}

func (h *MQHandler) dispatch(msg *protocol.Message) {
	cid, conn := h.balancer.Next()
	if conn == nil {
		h.buffer.Push(msg) // Re-queue
		return
	}

	h.mu.Lock()
	msgID := fmt.Sprintf("%x", msg.MessageID)
	h.unackedMessages[msgID] = msg
	h.mu.Unlock()

	if err := msg.Encode(conn); err != nil {
		fmt.Printf("failed to send message to collector %s: %v\n", cid, err)
		h.balancer.Remove(cid)
		h.mu.Lock()
		delete(h.unackedMessages, msgID)
		h.mu.Unlock()
		h.buffer.Push(msg)
		return
	}
}

func (h *MQHandler) Handle(ctx context.Context, conn net.Conn, msg *protocol.Message) error {
	switch msg.Type {
	case protocol.TypeStreamerHandshake:
		return h.handleStreamerHandshake(conn, msg)
	case protocol.TypeCollectorHandshake:
		return h.handleCollectorHandshake(conn, msg)
	case protocol.TypeTelemetryData:
		return h.handleTelemetryData(conn, msg)
	case protocol.TypeACK:
		msgID := fmt.Sprintf("%x", msg.MessageID)
		h.mu.Lock()
		delete(h.unackedMessages, msgID)
		h.mu.Unlock()
		return nil
	default:
		return fmt.Errorf("unsupported message type: %v", msg.Type)
	}
}

func (h *MQHandler) handleStreamerHandshake(conn net.Conn, msg *protocol.Message) error {
	fmt.Printf("Streamer connected: %x\n", msg.StreamerID)
	ack := &protocol.Message{
		Type:       protocol.TypeACK,
		MessageID:  msg.MessageID,
		StreamerID: msg.StreamerID,
	}
	return ack.Encode(conn)
}

func (h *MQHandler) handleCollectorHandshake(conn net.Conn, msg *protocol.Message) error {
	collectorID := fmt.Sprintf("%x", msg.StreamerID)
	h.balancer.Add(collectorID, conn)
	fmt.Printf("Collector connected: %s\n", collectorID)

	ack := &protocol.Message{
		Type:       protocol.TypeACK,
		MessageID:  msg.MessageID,
		StreamerID: msg.StreamerID,
	}
	return ack.Encode(conn)
}

func (h *MQHandler) handleTelemetryData(conn net.Conn, msg *protocol.Message) error {
	if !h.buffer.Push(msg) {
		// Buffer full, send NACK (Back-pressure)
		nack := &protocol.Message{
			Type:       protocol.TypeNACK,
			MessageID:  msg.MessageID,
			StreamerID: msg.StreamerID,
		}
		return nack.Encode(conn)
	}

	// ACK receipt to streamer
	ack := &protocol.Message{
		Type:       protocol.TypeACK,
		MessageID:  msg.MessageID,
		StreamerID: msg.StreamerID,
	}
	return ack.Encode(conn)
}
