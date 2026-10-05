package server

import (
	"crypto/tls"
	"log"
	"net"
	"time"

	"github.com/ozodmeofficial/smarteye/internal/protocol"
	"github.com/ozodmeofficial/smarteye/internal/security"
)

// acceptAgents listens for TLS agent connections and serves each one.
func (s *Server) acceptAgents(ln net.Listener) {
	for {
		raw, err := ln.Accept()
		if err != nil {
			if s.closing() {
				return
			}
			log.Printf("server: accept: %v", err)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		go s.serveAgent(raw)
	}
}

// serveAgent runs the handshake and read loop for one agent.
func (s *Server) serveAgent(raw net.Conn) {
	conn := protocol.NewConn(raw)
	defer conn.Close()

	// Handshake: expect Hello within a short window.
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	env, err := conn.Recv()
	if err != nil || env.Type != protocol.TypeHello {
		return
	}
	var hello protocol.Hello
	if err := env.Decode(&hello); err != nil {
		return
	}

	// Verify protocol major version and pairing code.
	if !sameMajor(hello.Version, protocol.Version) {
		_ = conn.SendTyped(protocol.TypeReject, protocol.Reject{Reason: "version_mismatch", Detail: hello.Version})
		return
	}
	if !security.VerifyNetCode(hello.NetCode, s.codeHash) {
		_ = conn.SendTyped(protocol.TypeReject, protocol.Reject{Reason: "bad_code"})
		log.Printf("server: rejected %s (%s): wrong network code", hello.Hostname, raw.RemoteAddr())
		return
	}
	conn.SetReadDeadline(time.Time{})

	addr := raw.RemoteAddr().String()
	d := s.hub.register(hello, addr, conn)
	log.Printf("server: device online: %s (%s) @ %s", d.Hostname, d.ID[:8], addr)

	_ = conn.SendTyped(protocol.TypeWelcome, protocol.Welcome{
		ServerName: s.cfg.ServerName,
		DeviceID:   d.ID,
		Room:       d.Room,
	})
	s.hub.enableThumbForConn(conn)
	s.hub.broadcastState()

	defer s.hub.unregister(hello.DeviceID, conn)

	// Read loop.
	for {
		env, err := conn.Recv()
		if err != nil {
			return
		}
		s.handleAgentMsg(hello.DeviceID, conn, env)
	}
}

func (s *Server) handleAgentMsg(id string, conn *protocol.Conn, env *protocol.Envelope) {
	switch env.Type {
	case protocol.TypePong:
		s.hub.setLatency(id, env.Latency().Milliseconds())

	case protocol.TypePing:
		_ = conn.SendTyped(protocol.TypePong, nil)

	case protocol.TypeClientUpdate:
		var u protocol.ClientUpdate
		env.Decode(&u)
		s.hub.updateStatus(id, u)

	case protocol.TypeMonitorList:
		var m protocol.MonitorList
		env.Decode(&m)
		s.hub.setMonitors(id, m.Monitors)

	case protocol.TypeThumbFrame:
		var f protocol.Frame
		env.Decode(&f)
		s.hub.setThumb(id, f)

	case protocol.TypeStreamFrame:
		var f protocol.Frame
		env.Decode(&f)
		s.hub.forwardStreamFrame(id, f)

	case protocol.TypeClipboard:
		var c protocol.Clipboard
		env.Decode(&c)
		// Mirror the client's clipboard to viewers (shared clipboard).
		s.hub.pushToAll(browserMsg{Type: "clipboard", Payload: mustJSON(map[string]any{
			"id": id, "text": c.Text,
		})})
	}
}

// pingLoop periodically pings every connected agent to measure latency and
// detect dead links.
func (s *Server) pingLoop() {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-t.C:
			s.hub.mu.RLock()
			conns := make([]*protocol.Conn, 0, len(s.hub.devices))
			for _, d := range s.hub.devices {
				if d.conn != nil {
					conns = append(conns, d.conn)
				}
			}
			s.hub.mu.RUnlock()
			for _, c := range conns {
				_ = c.SendTyped(protocol.TypePing, nil)
			}
		}
	}
}

func sameMajor(a, b string) bool {
	return majorOf(a) == majorOf(b)
}

func majorOf(v string) string {
	for i := 0; i < len(v); i++ {
		if v[i] == '.' {
			return v[:i]
		}
	}
	return v
}

// agentTLSConfig builds the server TLS config from its certificate.
func agentTLSConfig(cert tls.Certificate) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
}
