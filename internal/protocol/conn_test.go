package protocol

import (
	"net"
	"testing"
	"time"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	hello := Hello{DeviceID: "abc", Hostname: "pc-1", Version: Version}
	env, err := NewEnvelope(TypeHello, hello)
	if err != nil {
		t.Fatal(err)
	}
	var got Hello
	if err := env.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.DeviceID != "abc" || got.Hostname != "pc-1" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestConnSendRecv(t *testing.T) {
	c1, c2 := net.Pipe()
	a, b := NewConn(c1), NewConn(c2)
	defer a.Close()
	defer b.Close()

	go func() {
		_ = a.SendTyped(TypeLock, Lock{Title: "t", Message: "m"})
	}()

	b.SetReadDeadline(time.Now().Add(2 * time.Second))
	env, err := b.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if env.Type != TypeLock {
		t.Fatalf("want lock, got %s", env.Type)
	}
	var l Lock
	if err := env.Decode(&l); err != nil {
		t.Fatal(err)
	}
	if l.Title != "t" || l.Message != "m" {
		t.Fatalf("payload mismatch: %+v", l)
	}
}

func TestFrameTooLargeRejected(t *testing.T) {
	c1, c2 := net.Pipe()
	a := NewConn(c1)
	defer a.Close()
	defer c2.Close()
	// Oversized frames must be refused before hitting the wire.
	env := &Envelope{Type: TypeThumbFrame}
	env.Payload = make([]byte, MaxFrameSize+1)
	go func() { _ = c2.SetReadDeadline(time.Now().Add(time.Second)); buf := make([]byte, 8); _, _ = c2.Read(buf) }()
	if err := a.Send(env); err == nil {
		t.Fatal("expected error sending oversized frame")
	}
}
