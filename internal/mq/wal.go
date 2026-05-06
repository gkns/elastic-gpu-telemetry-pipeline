package mq

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Checkpoint stores the last known safe position for each streamer's WAL.
type Checkpoint struct {
	TS        int64                       `json:"ts"`
	Streamers map[string]*StreamerOffset  `json:"streamers"`
}

type StreamerOffset struct {
	LastSeq   uint64 `json:"last_seq"`
	WALOffset int64  `json:"wal_offset"`
}

// WAL is a per-streamer append-only write-ahead log.
type WAL struct {
	dir string
}

func NewWAL(dir string) (*WAL, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create wal dir: %w", err)
	}
	return &WAL{dir: dir}, nil
}

// walPath returns the path for a given streamer's WAL file.
func (w *WAL) walPath(streamerID string) string {
	return filepath.Join(w.dir, streamerID+".wal")
}

// checkpointPath returns the path for the checkpoint file.
func (w *WAL) checkpointPath() string {
	return filepath.Join(w.dir, "checkpoint.json")
}

// Append writes one WAL entry for the given message.
// Entry layout: SeqNum(8B) | StreamerIDLen(2B) | StreamerID | PayloadLen(4B) | Payload
func (w *WAL) Append(streamerID string, seqNum uint64, payload []byte) error {
	f, err := os.OpenFile(w.walPath(streamerID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open wal: %w", err)
	}
	defer f.Close()

	idBytes := []byte(streamerID)
	entry := make([]byte, 8+2+len(idBytes)+4+len(payload))
	off := 0
	binary.BigEndian.PutUint64(entry[off:], seqNum)
	off += 8
	binary.BigEndian.PutUint16(entry[off:], uint16(len(idBytes)))
	off += 2
	copy(entry[off:], idBytes)
	off += len(idBytes)
	binary.BigEndian.PutUint32(entry[off:], uint32(len(payload)))
	off += 4
	copy(entry[off:], payload)

	_, err = f.Write(entry)
	return err
}

// WriteCheckpoint persists the checkpoint JSON.
func (w *WAL) WriteCheckpoint(cp *Checkpoint) error {
	cp.TS = time.Now().Unix()
	b, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	return os.WriteFile(w.checkpointPath(), b, 0o644)
}

// ReadCheckpoint loads the last checkpoint, returning an empty one if missing.
func (w *WAL) ReadCheckpoint() (*Checkpoint, error) {
	b, err := os.ReadFile(w.checkpointPath())
	if os.IsNotExist(err) {
		return &Checkpoint{Streamers: make(map[string]*StreamerOffset)}, nil
	}
	if err != nil {
		return nil, err
	}
	cp := &Checkpoint{}
	if err := json.Unmarshal(b, cp); err != nil {
		return nil, err
	}
	if cp.Streamers == nil {
		cp.Streamers = make(map[string]*StreamerOffset)
	}
	return cp, nil
}

// ReplayFrom reads WAL entries for each streamer since the checkpoint offset
// and returns them as InFlightMessages ready for redelivery.
func (w *WAL) ReplayFrom(cp *Checkpoint) ([]*InFlightMessage, error) {
	entries, err := filepath.Glob(filepath.Join(w.dir, "*.wal"))
	if err != nil {
		return nil, err
	}

	var msgs []*InFlightMessage
	for _, path := range entries {
		base := filepath.Base(path)
		streamerID := base[:len(base)-4] // strip .wal

		offset := int64(0)
		if so, ok := cp.Streamers[streamerID]; ok {
			offset = so.WALOffset
		}

		f, err := os.Open(path)
		if err != nil {
			continue
		}
		if offset > 0 {
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				f.Close()
				continue
			}
		}

		for {
			var seqNum uint64
			if err := binary.Read(f, binary.BigEndian, &seqNum); err != nil {
				break
			}
			var idLen uint16
			if err := binary.Read(f, binary.BigEndian, &idLen); err != nil {
				break
			}
			idBuf := make([]byte, idLen)
			if _, err := io.ReadFull(f, idBuf); err != nil {
				break
			}
			var payLen uint32
			if err := binary.Read(f, binary.BigEndian, &payLen); err != nil {
				break
			}
			payload := make([]byte, payLen)
			if _, err := io.ReadFull(f, payload); err != nil {
				break
			}
			msgs = append(msgs, &InFlightMessage{
				MsgID:      seqNum, // use seqNum as temporary MsgID for replay
				StreamerID: string(idBuf),
				SeqNum:     seqNum,
				SentAt:     time.Now(),
				Payload:    payload,
			})
		}
		f.Close()
	}
	return msgs, nil
}
