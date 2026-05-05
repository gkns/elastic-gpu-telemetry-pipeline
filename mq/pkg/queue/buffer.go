package queue

import (
	"sync"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/protocol"
)

type Buffer struct {
	messages []*protocol.Message
	mu       sync.Mutex
	cond     *sync.Cond
	capacity int
}

func NewBuffer(capacity int) *Buffer {
	b := &Buffer{
		messages: make([]*protocol.Message, 0, capacity),
		capacity: capacity,
	}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *Buffer) Push(msg *protocol.Message) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.messages) >= b.capacity {
		return false
	}

	b.messages = append(b.messages, msg)
	b.cond.Signal()
	return true
}

func (b *Buffer) Pop() *protocol.Message {
	b.mu.Lock()
	defer b.mu.Unlock()

	for len(b.messages) == 0 {
		b.cond.Wait()
	}

	msg := b.messages[0]
	b.messages = b.messages[1:]
	return msg
}

func (b *Buffer) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.messages)
}
