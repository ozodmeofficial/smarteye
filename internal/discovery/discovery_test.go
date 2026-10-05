package discovery

import (
	"context"
	"testing"
	"time"
)

func TestExpandIPs(t *testing.T) {
	got := expandIPs([]string{"192.168.1.20", "192.168.1.30-32", "bad", "10.0.0.1"})
	want := []string{"192.168.1.20", "192.168.1.30", "192.168.1.31", "192.168.1.32", "10.0.0.1"}
	if len(got) != len(want) {
		t.Fatalf("got %d ips, want %d: %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].String() != w {
			t.Fatalf("ip[%d] = %s, want %s", i, got[i], w)
		}
	}
}

// TestProbeReachesListener verifies the manual "search by IP" unicast beacon is
// received by a listening client on loopback.
func TestProbeReachesListener(t *testing.T) {
	const port = 47977
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	l := &Listener{Port: port}
	ch, err := l.Listen(ctx, "codehash-xyz")
	if err != nil {
		t.Fatal(err)
	}
	// Give the listener a moment to bind before probing.
	time.Sleep(100 * time.Millisecond)

	go Probe(port, []string{"127.0.0.1"}, Beacon{
		ServerName: "srv", Port: 47800, CodeHash: "codehash-xyz", Version: "1.0.0",
	})

	select {
	case found, ok := <-ch:
		if !ok {
			t.Fatal("listener channel closed without a beacon")
		}
		if found.Beacon.CodeHash != "codehash-xyz" || found.Beacon.Port != 47800 {
			t.Fatalf("unexpected beacon: %+v", found.Beacon)
		}
	case <-ctx.Done():
		t.Fatal("did not receive unicast probe beacon in time")
	}
}
