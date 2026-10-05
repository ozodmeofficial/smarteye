// Package discovery implements zero-configuration LAN discovery so a client
// finds its server automatically, with no IP typed by hand.
//
// The server broadcasts a small UDP beacon on the discovery port a few times a
// second. Every beacon carries the server's WebSocket address and a *hash* of
// the shared pairing code. A client listens for beacons; when it sees one whose
// code hash matches its own, it connects to the advertised address. The code
// itself is never broadcast.
//
// Everything stays on the local subnet: beacons go to the broadcast address and
// are never routed to the internet.
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Magic identifies a SmartEYE beacon so unrelated UDP traffic is ignored.
const Magic = "SMARTEYE-BEACON-1"

// Beacon is the payload the server broadcasts.
type Beacon struct {
	Magic      string `json:"magic"`
	ServerName string `json:"server_name"`
	Host       string `json:"host"`      // IP clients should connect to
	Port       int    `json:"port"`      // WebSocket TLS port
	CodeHash   string `json:"code_hash"` // salted hash of the pairing code
	Version    string `json:"version"`
}

// Broadcaster periodically announces a server on the LAN.
type Broadcaster struct {
	Port     int           // discovery UDP port
	Interval time.Duration // how often to beacon (default 2s)
	Beacon   Beacon
}

// Run broadcasts until ctx is cancelled. It re-resolves the broadcast address
// each tick so it keeps working across network changes (e.g. Wi-Fi reconnect).
func (b *Broadcaster) Run(ctx context.Context) error {
	interval := b.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return fmt.Errorf("discovery: open broadcast socket: %w", err)
	}
	defer conn.Close()

	b.Beacon.Magic = Magic
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	send := func() {
		b.Beacon.Host = primaryIPv4()
		data, err := json.Marshal(b.Beacon)
		if err != nil {
			return
		}
		for _, dst := range broadcastTargets(b.Port) {
			_, _ = conn.WriteToUDP(data, dst)
		}
	}

	send() // immediate first beacon
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			send()
		}
	}
}

// Listener watches for server beacons on the LAN.
type Listener struct {
	Port int
}

// Found is a beacon together with the source address it arrived from.
type Found struct {
	Beacon Beacon
	From   *net.UDPAddr
}

// Listen delivers matching beacons to the returned channel until ctx is
// cancelled. Only beacons carrying wantCodeHash are forwarded; pass an empty
// string to receive every SmartEYE beacon (used by the manual "scan" UI).
func (l *Listener) Listen(ctx context.Context, wantCodeHash string) (<-chan Found, error) {
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: l.Port}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("discovery: listen on %d: %w", l.Port, err)
	}
	out := make(chan Found, 16)
	go func() {
		defer close(out)
		defer conn.Close()
		buf := make([]byte, 2048)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			n, from, err := conn.ReadFromUDP(buf)
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue
				}
				return
			}
			var bc Beacon
			if json.Unmarshal(buf[:n], &bc) != nil || bc.Magic != Magic {
				continue
			}
			if wantCodeHash != "" && bc.CodeHash != wantCodeHash {
				continue // beacon from a different SmartEYE fleet
			}
			// Prefer the host the beacon advertised; fall back to the sender IP.
			if bc.Host == "" || bc.Host == "0.0.0.0" {
				bc.Host = from.IP.String()
			}
			select {
			case out <- Found{Beacon: bc, From: from}:
			case <-ctx.Done():
				return
			default: // drop if the consumer is slow; next beacon comes soon
			}
		}
	}()
	return out, nil
}

// broadcastTargets returns the addresses a beacon should be sent to: the global
// broadcast plus each interface's directed broadcast, improving delivery on
// segmented networks.
func broadcastTargets(port int) []*net.UDPAddr {
	targets := []*net.UDPAddr{{IP: net.IPv4bcast, Port: port}}
	ifaces, err := net.Interfaces()
	if err != nil {
		return targets
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagBroadcast == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}
			if bc := directedBroadcast(ipnet); bc != nil {
				targets = append(targets, &net.UDPAddr{IP: bc, Port: port})
			}
		}
	}
	return targets
}

// directedBroadcast computes the broadcast address for an IPv4 network.
func directedBroadcast(n *net.IPNet) net.IP {
	ip := n.IP.To4()
	if ip == nil {
		return nil
	}
	mask := n.Mask
	bc := make(net.IP, len(ip))
	for i := range ip {
		bc[i] = ip[i] | ^mask[i]
	}
	return bc
}

// primaryIPv4 returns the most likely LAN IPv4 address of this host.
func primaryIPv4() string {
	// A UDP "dial" to a non-routable address reveals the outbound interface IP
	// without sending anything.
	conn, err := net.Dial("udp4", "192.168.255.255:9")
	if err == nil {
		defer conn.Close()
		if la, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return la.IP.String()
		}
	}
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					return ip4.String()
				}
			}
		}
	}
	return "0.0.0.0"
}
