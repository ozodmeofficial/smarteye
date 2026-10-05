package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ozodmeofficial/smarteye/internal/config"
	"github.com/ozodmeofficial/smarteye/internal/discovery"
	"github.com/ozodmeofficial/smarteye/internal/meta"
	"github.com/ozodmeofficial/smarteye/internal/security"
)

// Server is the SmartEYE control-side process.
type Server struct {
	cfg      *config.Config
	codeHash string
	cert     tls.Certificate
	hub      *Hub

	httpSrv *http.Server
	done    chan struct{}
	closed  atomic.Bool

	// UIAddr is the local http address the dashboard is served on.
	UIAddr string

	once sync.Once
}

// New constructs a server from config, generating TLS material and loading the
// persisted room layout.
func New(cfg *config.Config) (*Server, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	cert, err := security.EnsureServerCert(dir)
	if err != nil {
		return nil, err
	}
	s := &Server{
		cfg:      cfg,
		codeHash: security.HashNetCode(config.NormalizeNetCode(cfg.NetCode)),
		cert:     cert,
		hub:      NewHub(newRoomStore(dir)),
		done:     make(chan struct{}),
	}
	return s, nil
}

// Run starts the agent listener, discovery beacon, ping loop and the local web
// dashboard. It blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	// 1. Agent TLS listener.
	agentAddr := ":" + strconv.Itoa(s.cfg.ListenPort)
	ln, err := tls.Listen("tcp", agentAddr, agentTLSConfig(s.cert))
	if err != nil {
		return fmt.Errorf("server: listen agents on %s: %w", agentAddr, err)
	}
	log.Printf("server: accepting agents on %s", agentAddr)
	go s.acceptAgents(ln)
	go s.pingLoop()

	// 2. LAN discovery beacon so clients find us with no IP typed.
	go func() {
		b := &discovery.Broadcaster{
			Port: s.cfg.DiscoveryPort,
			Beacon: discovery.Beacon{
				ServerName: s.cfg.ServerName,
				Port:       s.cfg.ListenPort,
				CodeHash:   s.codeHash,
				Version:    meta.Version,
			},
		}
		if err := b.Run(ctx); err != nil {
			log.Printf("server: discovery beacon stopped: %v", err)
		}
	}()

	// 3. Local web dashboard (loopback only; the operator uses it in a browser
	//    or the bundled webview).
	uiLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("server: listen ui: %w", err)
	}
	s.UIAddr = "http://" + uiLn.Addr().String()
	s.httpSrv = &http.Server{Handler: s.router()}
	log.Printf("server: dashboard at %s", s.UIAddr)
	go func() {
		if err := s.httpSrv.Serve(uiLn); err != nil && err != http.ErrServerClosed {
			log.Printf("server: ui http: %v", err)
		}
	}()

	<-ctx.Done()
	s.shutdown(ln)
	return ctx.Err()
}

func (s *Server) shutdown(agentLn net.Listener) {
	s.once.Do(func() {
		s.closed.Store(true)
		close(s.done)
		_ = agentLn.Close()
		if s.httpSrv != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = s.httpSrv.Shutdown(ctx)
		}
	})
}

func (s *Server) closing() bool { return s.closed.Load() }
