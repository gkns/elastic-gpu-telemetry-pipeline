package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/protocol"
)

type Handler interface {
	Handle(ctx context.Context, conn net.Conn, msg *protocol.Message) error
}

type Server struct {
	addr     string
	listener net.Listener
	handler  Handler
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewServer(addr string, handler Handler) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		addr:    addr,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (s *Server) Start() error {
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", s.addr, err)
	}
	s.listener = l

	fmt.Printf("MQ Server listening on %s\n", s.addr)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil
			default:
				fmt.Printf("failed to accept connection: %v\n", err)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) Stop() {
	s.cancel()
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			msg, err := protocol.Decode(conn)
			if err != nil {
				if err != io.EOF && err != net.ErrClosed {
					fmt.Printf("failed to decode message from %s: %v\n", conn.RemoteAddr(), err)
				}
				return
			}

			if err := s.handler.Handle(s.ctx, conn, msg); err != nil {
				fmt.Printf("failed to handle message from %s: %v\n", conn.RemoteAddr(), err)
				return
			}
		}
	}
}
