package protocol

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// MaxFrameSize caps a single message so a buggy or hostile peer cannot force an
// unbounded allocation. 32 MiB comfortably holds a full-screen JPEG frame.
const MaxFrameSize = 32 << 20

// Conn is a length-prefixed JSON message connection used between the SmartEYE
// agent and server over a TLS TCP socket. Each frame is a 4-byte big-endian
// length followed by that many bytes of JSON (an Envelope).
//
// Conn is safe for one concurrent reader and one concurrent writer. Writes are
// serialized with a mutex so multiple goroutines can send.
type Conn struct {
	raw net.Conn
	r   *bufio.Reader
	w   *bufio.Writer
	wmu sync.Mutex
}

// NewConn wraps a net.Conn (typically *tls.Conn) as a framed message Conn.
func NewConn(raw net.Conn) *Conn {
	return &Conn{
		raw: raw,
		r:   bufio.NewReaderSize(raw, 64<<10),
		w:   bufio.NewWriterSize(raw, 64<<10),
	}
}

// Send writes an envelope. It is safe to call from multiple goroutines.
func (c *Conn) Send(e *Envelope) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("protocol: marshal envelope: %w", err)
	}
	if len(data) > MaxFrameSize {
		return fmt.Errorf("protocol: frame too large (%d bytes)", len(data))
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(data)))

	c.wmu.Lock()
	defer c.wmu.Unlock()
	if _, err := c.w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := c.w.Write(data); err != nil {
		return err
	}
	return c.w.Flush()
}

// SendTyped is a convenience that builds and sends an envelope in one call.
func (c *Conn) SendTyped(t MessageType, payload any) error {
	e, err := NewEnvelope(t, payload)
	if err != nil {
		return err
	}
	return c.Send(e)
}

// Recv reads the next envelope. It blocks until a full frame arrives, the
// deadline fires, or the connection closes.
func (c *Conn) Recv() (*Envelope, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(c.r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > MaxFrameSize {
		return nil, fmt.Errorf("protocol: incoming frame too large (%d bytes)", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(c.r, buf); err != nil {
		return nil, err
	}
	var e Envelope
	if err := json.Unmarshal(buf, &e); err != nil {
		return nil, fmt.Errorf("protocol: unmarshal envelope: %w", err)
	}
	return &e, nil
}

// SetReadDeadline sets the deadline for future Recv calls.
func (c *Conn) SetReadDeadline(t time.Time) error { return c.raw.SetReadDeadline(t) }

// SetWriteDeadline sets the deadline for future Send calls.
func (c *Conn) SetWriteDeadline(t time.Time) error { return c.raw.SetWriteDeadline(t) }

// RemoteAddr returns the peer address.
func (c *Conn) RemoteAddr() net.Addr { return c.raw.RemoteAddr() }

// Close closes the underlying connection.
func (c *Conn) Close() error { return c.raw.Close() }
