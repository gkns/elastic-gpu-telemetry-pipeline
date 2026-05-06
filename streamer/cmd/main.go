package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/streamer/pkg/client"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/streamer/pkg/processor"
	"github.com/gkns/elastic-gpu-telemetry-pipeline/streamer/pkg/reader"
)

func main() {
	mqAddr := flag.String("mq", "localhost:9090", "MQ server address")
	csvPath := flag.String("input", "", "Path to CSV file")
	interval := flag.Duration("interval", time.Second, "Streaming interval")
	flag.Parse()

	if *csvPath == "" {
		log.Fatal("input CSV path is required")
	}

	streamerID := [16]byte{0x01} // Fixed ID for MVP

	mqClient, err := client.NewMQClient(*mqAddr, streamerID)
	if err != nil {
		log.Fatalf("failed to connect to MQ: %v", err)
	}
	defer mqClient.Close()

	csvReader, err := reader.NewCSVReader(*csvPath)
	if err != nil {
		log.Fatalf("failed to open CSV: %v", err)
	}
	defer csvReader.Close()

	fmt.Printf("Streaming telemetry from %s to %s...\n", *csvPath, *mqAddr)

	for {
		row, err := csvReader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				fmt.Println("reached end of CSV, restarting...")
				csvReader.Close()
				csvReader, _ = reader.NewCSVReader(*csvPath)
				continue
			}
			log.Printf("failed to read row: %v", err)
			break
		}

		processor.InjectTimestamp(row)

		data, _ := json.Marshal(row)
		if err := mqClient.Send(data); err != nil {
			log.Printf("failed to send data: %v", err)
		}

		time.Sleep(*interval)
	}
}
