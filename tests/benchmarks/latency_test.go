package benchmarks

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/protocol"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/queue"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/server"
)

func BenchmarkMQHopLatency(b *testing.B) {
	addr := "localhost:9090"
	buffer := queue.NewBuffer(1000)
	handler := server.NewMQHandler(buffer)
	srv := server.NewServer(addr, handler)

	go srv.Start()
	defer srv.Stop()

	// Wait for server
	time.Sleep(100 * time.Millisecond)

	// Streamer
	streamerConn, _ := net.Dial("tcp", addr)
	defer streamerConn.Close()

	// Handshake
	sid := [16]byte{0x01}
	handshake := &protocol.Message{Type: protocol.TypeStreamerHandshake, StreamerID: sid}
	handshake.Encode(streamerConn)
	protocol.Decode(streamerConn)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		msg := &protocol.Message{
			Type:       protocol.TypeTelemetryData,
			StreamerID: sid,
			Payload:    []byte("benchmark data"),
		}
		msg.Encode(streamerConn)
		
		ack, _ := protocol.Decode(streamerConn)
		if ack.Type != protocol.TypeACK {
			b.Errorf("expected ACK, got %v", ack.Type)
		}
		
		latency := time.Since(start)
		if latency > 50*time.Millisecond {
			// SC-001 violation
		}
	}
}
