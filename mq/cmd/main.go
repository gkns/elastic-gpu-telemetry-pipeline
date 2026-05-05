package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/queue"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/server"
)

func main() {
	addr := flag.String("addr", ":8080", "Listen address")
	snapshotPath := flag.String("snapshot", "mq_state.bin", "Path to snapshot file")
	flag.Parse()

	buffer := queue.NewBuffer(1000)

	// Restore state
	if err := buffer.Restore(*snapshotPath); err != nil {
		log.Printf("failed to restore snapshot: %v", err)
	}

	handler := server.NewMQHandler(buffer)
	loggingHandler := server.NewLoggingMiddleware(handler)
	srv := server.NewServer(*addr, loggingHandler)

	// Periodic snapshot
	go func() {
		for {
			time.Sleep(30 * time.Second)
			if err := buffer.Snapshot(*snapshotPath); err != nil {
				log.Printf("periodic snapshot failed: %v", err)
			}
		}
	}()

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("server failed: %v", err)
		}
	}()

	fmt.Printf("MQ Server started on %s\n", *addr)

	// Wait for signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Shutting down...")
	srv.Stop()

	// Final snapshot
	if err := buffer.Snapshot(*snapshotPath); err != nil {
		log.Printf("final snapshot failed: %v", err)
	}
}
