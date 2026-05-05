package queue

import (
	"net"
	"sync"
)

type Balancer struct {
	collectors     map[string]net.Conn
	collectorOrder []string
	next           int
	mu             sync.Mutex
}

func NewBalancer() *Balancer {
	return &Balancer{
		collectors: make(map[string]net.Conn),
	}
}

func (b *Balancer) Add(id string, conn net.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.collectors[id] = conn
	b.updateOrder()
}

func (b *Balancer) Remove(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.collectors, id)
	b.updateOrder()
}

func (b *Balancer) Next() (string, net.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.collectorOrder) == 0 {
		return "", nil
	}

	if b.next >= len(b.collectorOrder) {
		b.next = 0
	}

	id := b.collectorOrder[b.next]
	conn := b.collectors[id]
	b.next++

	return id, conn
}

func (b *Balancer) updateOrder() {
	b.collectorOrder = make([]string, 0, len(b.collectors))
	for id := range b.collectors {
		b.collectorOrder = append(b.collectorOrder, id)
	}
}
