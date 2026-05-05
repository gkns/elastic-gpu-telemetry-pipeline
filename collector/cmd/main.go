package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/collector/pkg/client"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/collector/pkg/store"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/streamer/pkg/reader"
)

func main() {
	mqAddr := flag.String("mq", "localhost:8080", "MQ server address")
	dbConn := flag.String("db", "postgres://admin:quest@localhost:8812/qdb", "QuestDB connection string")
	flag.Parse()

	collectorID := [16]byte{0x02} // Fixed ID for MVP

	db, err := store.NewDB(*dbConn)
	if err != nil {
		log.Fatalf("failed to connect to QuestDB: %v", err)
	}
	defer db.Close()

	// Wait for QuestDB to be ready
	for i := 0; i < 10; i++ {
		if err := db.Ping(context.Background()); err == nil {
			break
		}
		fmt.Println("waiting for QuestDB...")
		time.Sleep(2 * time.Second)
	}

	mqClient, err := client.NewMQClient(*mqAddr, collectorID)
	if err != nil {
		log.Fatalf("failed to connect to MQ: %v", err)
	}
	defer mqClient.Close()

	fmt.Printf("Collector connected to MQ %s and QuestDB %s\n", *mqAddr, *dbConn)

	for {
		msg, err := mqClient.Receive()
		if err != nil {
			log.Printf("failed to receive message: %v", err)
			break
		}

		var row reader.TelemetryRow
		if err := json.Unmarshal(msg.Payload, &row); err != nil {
			log.Printf("failed to unmarshal payload: %v", err)
			continue
		}

		if err := db.InsertTelemetry(context.Background(), &row); err != nil {
			log.Printf("failed to insert telemetry: %v", err)
			continue
		}

		fmt.Printf("Persisted telemetry: %s %s=%s\n", row.UUID, row.MetricName, row.Value)

		if err := mqClient.ACK(msg.MessageID, msg.StreamerID); err != nil {
			log.Printf("failed to send ACK: %v", err)
		}
	}
}
