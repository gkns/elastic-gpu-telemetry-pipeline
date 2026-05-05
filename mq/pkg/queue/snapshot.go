package queue

import (
	"encoding/gob"
	"fmt"
	"os"

	"github.com/gkns/elastic-gpu-telemetry-pipeline/mq/pkg/protocol"
)

func (b *Buffer) Snapshot(filePath string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	if err := enc.Encode(b.messages); err != nil {
		return err
	}

	fmt.Printf("Snapshot saved to %s (%d messages)\n", filePath, len(b.messages))
	return nil
}

func (b *Buffer) Restore(filePath string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	var messages []*protocol.Message
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&messages); err != nil {
		return err
	}

	b.messages = messages
	fmt.Printf("Restored %d messages from %s\n", len(b.messages), filePath)
	return nil
}
