package protocol

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

var bufPool = sync.Pool{
	New: func() any { b := make([]byte, 4096); return &b },
}

// ReadFrame reads one frame from r.
// Header layout: Type(1B) + Length(4B) + MsgID(8B)
func ReadFrame(r *bufio.Reader) (*Frame, error) {
	hdr := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	f := &Frame{
		Type:  MsgType(hdr[0]),
		MsgID: binary.BigEndian.Uint64(hdr[5:13]),
	}
	bodyLen := int(binary.BigEndian.Uint32(hdr[1:5]))
	if bodyLen > 0 {
		f.Body = make([]byte, bodyLen)
		if _, err := io.ReadFull(r, f.Body); err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}
	}
	return f, nil
}

// WriteFrame writes one frame to w.
func WriteFrame(w io.Writer, f *Frame) error {
	bp := bufPool.Get().(*[]byte)
	defer bufPool.Put(bp)

	need := HeaderSize + len(f.Body)
	buf := *bp
	if len(buf) < need {
		buf = make([]byte, need)
		*bp = buf
	}
	buf = buf[:need]
	buf[0] = byte(f.Type)
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(f.Body)))
	binary.BigEndian.PutUint64(buf[5:13], f.MsgID)
	copy(buf[HeaderSize:], f.Body)
	_, err := w.Write(buf)
	return err
}
