package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/protocol"
)

type LoggingMiddleware struct {
	next Handler
}

func NewLoggingMiddleware(next Handler) *LoggingMiddleware {
	return &LoggingMiddleware{next: next}
}

func (m *LoggingMiddleware) Handle(ctx context.Context, conn net.Conn, msg *protocol.Message) error {
	start := time.Now()
	err := m.next.Handle(ctx, conn, msg)
	duration := time.Since(start)

	fmt.Printf("[%s] %s Type=%s Length=%d Duration=%v Error=%v\n",
		time.Now().Format(time.RFC3339),
		conn.RemoteAddr(),
		msg.Type.String(),
		msg.TotalLength,
		duration,
		err,
	)

	return err
}
