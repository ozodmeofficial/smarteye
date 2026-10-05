// Package agent implements the SmartEYE client: the program that runs on a
// student's or employee's computer. It auto-discovers its server on the LAN,
// connects over TLS, streams a live preview, and carries out supervision and
// remote-control commands. Supervision is deliberately visible: the machine
// reports its state and (by config default) shows an indicator when watched.
package agent

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/ozodmeofficial/smarteye/internal/config"
	"github.com/ozodmeofficial/smarteye/internal/control"
	"github.com/ozodmeofficial/smarteye/internal/discovery"
	"github.com/ozodmeofficial/smarteye/internal/meta"
	"github.com/ozodmeofficial/smarteye/internal/osops"
	"github.com/ozodmeofficial/smarteye/internal/protocol"
	"github.com/ozodmeofficial/smarteye/internal/screen"
	"github.com/ozodmeofficial/smarteye/internal/security"
)

// Agent is the running client.
type Agent struct {
	cfg      *config.Config
	codeHash string

	cap    screen.Capturer
	inj    control.Injector
	locker osops.Locker

	// Per-connection state, reset on each (re)connect.
	conn     *protocol.Conn
	streamer *streamer
}

// New builds an agent from config.
func New(cfg *config.Config) (*Agent, error) {
	cap, err := screen.New()
	if err != nil {
		return nil, err
	}
	return &Agent{
		cfg:      cfg,
		codeHash: security.HashNetCode(config.NormalizeNetCode(cfg.NetCode)),
		cap:      cap,
		inj:      control.New(),
		locker:   osops.NewLocker(),
	}, nil
}

// Run keeps the agent connected to its server forever: discover, connect,
// serve, and on any drop, reconnect with a short backoff. This loop is the
// agent's watchdog against transient network failures.
func (a *Agent) Run(ctx context.Context) error {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		addr, err := a.locate(ctx)
		if err != nil {
			log.Printf("agent: locate server: %v", err)
			if !sleep(ctx, backoff) {
				return ctx.Err()
			}
			backoff = nextBackoff(backoff)
			continue
		}
		log.Printf("agent: connecting to %s", addr)
		if err := a.serve(ctx, addr); err != nil {
			log.Printf("agent: session ended: %v", err)
		}
		backoff = time.Second // reset after a successful session attempt
		if !sleep(ctx, 2*time.Second) {
			return ctx.Err()
		}
	}
}

// locate finds the server address, preferring auto-discovery and falling back
// to a manually configured host.
func (a *Agent) locate(ctx context.Context) (string, error) {
	if a.cfg.ServerHost != "" {
		host := a.cfg.ServerHost
		if _, _, err := net.SplitHostPort(host); err != nil {
			host = net.JoinHostPort(host, strconv.Itoa(a.cfg.ListenPort))
		}
		return host, nil
	}
	lctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	l := &discovery.Listener{Port: a.cfg.DiscoveryPort}
	ch, err := l.Listen(lctx, a.codeHash)
	if err != nil {
		return "", err
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case found, ok := <-ch:
		if !ok {
			return "", context.DeadlineExceeded
		}
		// Prefer the address the beacon actually arrived from: it is the
		// server's reachable IP on this subnet. The advertised Host can be
		// wrong on multi-homed servers (VirtualBox/VMware/Hyper-V adapters),
		// so it is only a fallback.
		host := found.Beacon.Host
		if found.From != nil && found.From.IP != nil && !found.From.IP.IsUnspecified() {
			host = found.From.IP.String()
		}
		return net.JoinHostPort(host, strconv.Itoa(found.Beacon.Port)), nil
	}
}

// serve runs one full session with the server.
func (a *Agent) serve(ctx context.Context, addr string) error {
	// The server uses a self-signed LAN certificate; trust is established by the
	// shared pairing code carried in Hello, not by a public CA. Pinning the
	// cert on first contact is a planned hardening step.
	tlsCfg := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}
	d := &net.Dialer{Timeout: 10 * time.Second}
	raw, err := tls.DialWithDialer(d, "tcp", addr, tlsCfg)
	if err != nil {
		return err
	}
	conn := protocol.NewConn(raw)
	defer conn.Close()
	a.conn = conn

	info := osops.Info()
	hello := protocol.Hello{
		Version:    protocol.Version,
		DeviceID:   a.cfg.DeviceID,
		Hostname:   info.Hostname,
		Username:   info.Username,
		OS:         info.OS,
		AppVersion: meta.Version,
		NetCode:    a.codeHash,
	}
	if err := conn.SendTyped(protocol.TypeHello, hello); err != nil {
		return err
	}

	// Await welcome/reject.
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	env, err := conn.Recv()
	if err != nil {
		return err
	}
	if env.Type == protocol.TypeReject {
		var r protocol.Reject
		env.Decode(&r)
		return &rejectError{r.Reason, r.Detail}
	}
	if env.Type != protocol.TypeWelcome {
		return &rejectError{"unexpected", string(env.Type)}
	}
	conn.SetReadDeadline(time.Time{})
	log.Printf("agent: accepted by server")

	sctx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.streamer = newStreamer(a.cap, conn.Send)
	go a.streamer.run(sctx)
	go a.reportStatus(sctx)
	go a.keepAlive(sctx)

	// Advertise monitors up front.
	if mons, err := a.cap.Monitors(); err == nil {
		_ = conn.SendTyped(protocol.TypeMonitorList, protocol.MonitorList{Monitors: mons})
	}

	return a.readLoop(sctx)
}

// readLoop dispatches incoming commands until the connection closes.
func (a *Agent) readLoop(ctx context.Context) error {
	for {
		env, err := a.conn.Recv()
		if err != nil {
			return err
		}
		a.handle(env)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func (a *Agent) handle(env *protocol.Envelope) {
	switch env.Type {
	case protocol.TypePing:
		_ = a.conn.SendTyped(protocol.TypePong, nil)

	case protocol.TypeThumbRequest:
		var r protocol.ThumbRequest
		env.Decode(&r)
		a.streamer.setThumb(r)

	case protocol.TypeStreamStart:
		var s protocol.StreamStart
		env.Decode(&s)
		a.streamer.setStream(true, s.Monitor, s.FPS, s.Quality)

	case protocol.TypeStreamStop:
		a.streamer.setStream(false, 0, 0, 0)

	case protocol.TypeSelectMon:
		var s protocol.SelectMonitor
		env.Decode(&s)
		a.streamer.selectMonitor(s.Monitor)

	case protocol.TypeControlMode:
		var m protocol.ControlMode
		env.Decode(&m)
		a.onControlMode(m)

	case protocol.TypeInputEvent:
		var ev protocol.InputEvent
		env.Decode(&ev)
		a.applyInput(ev)

	case protocol.TypeClipboard:
		var c protocol.Clipboard
		env.Decode(&c)
		_ = a.inj.SetClipboard(c.Text)

	case protocol.TypeLock:
		var l protocol.Lock
		env.Decode(&l)
		_ = a.locker.Show(l.Title, l.Message)
		a.pushStatus()

	case protocol.TypeUnlock:
		_ = a.locker.Hide()
		a.pushStatus()

	case protocol.TypeMessage:
		var m protocol.Message
		env.Decode(&m)
		_ = osops.ShowMessage(m.Title, m.Body, m.Timeout)

	case protocol.TypePower:
		var p protocol.Power
		env.Decode(&p)
		_ = osops.Power(p.Action, p.Delay)

	case protocol.TypeLaunch:
		var l protocol.Launch
		env.Decode(&l)
		_ = osops.Launch(l.Target, l.Args, l.IsURL)

	case protocol.TypeDemoFrame:
		// The server is pushing the teacher's screen; the UI layer would show
		// it fullscreen. Handled by the agent's demo overlay (see docs).

	case protocol.TypeFileOffer, protocol.TypeFileChunk, protocol.TypeFileDone, protocol.TypeFileAccept:
		a.handleFile(env)
	}
}

func (a *Agent) onControlMode(m protocol.ControlMode) {
	// Openness: when an operator starts controlling, tell the user, unless the
	// admin disabled the indicator.
	if m.Control && a.cfg.ShowBadge {
		_ = osops.ShowMessage("SmartEYE", "Boshqaruvchi kompyuteringizga ulandi.", 4)
	}
	// FreezeClient (disabling local input during control) is applied by the
	// input layer; documented as a Windows enhancement.
}

func (a *Agent) applyInput(ev protocol.InputEvent) {
	mons, err := a.cap.Monitors()
	if err != nil || len(mons) == 0 {
		return
	}
	a.streamer.mu.Lock()
	idx := a.streamer.monitor
	a.streamer.mu.Unlock()
	if idx < 0 || idx >= len(mons) {
		idx = 0
	}
	m := mons[idx]
	_ = a.inj.Apply(ev, m.Width, m.Height, 0, 0)
}

// reportStatus periodically pushes the client's live state to the server.
func (a *Agent) reportStatus(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.pushStatus()
		}
	}
}

func (a *Agent) pushStatus() {
	fg := osops.ActiveWindow()
	upd := protocol.ClientUpdate{
		ForegroundApp:   fg.App,
		ForegroundTitle: fg.Title,
		Locked:          a.locker.IsLocked(),
		BatteryPercent:  osops.Battery(),
	}
	_ = a.conn.SendTyped(protocol.TypeClientUpdate, upd)
}

// keepAlive sends a ping periodically so dead connections surface quickly.
func (a *Agent) keepAlive(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = a.conn.SendTyped(protocol.TypePing, nil)
		}
	}
}

// --- helpers ----------------------------------------------------------------

type rejectError struct{ reason, detail string }

func (e *rejectError) Error() string { return "server rejected: " + e.reason + " " + e.detail }

func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

func nextBackoff(d time.Duration) time.Duration {
	d *= 2
	if d > 15*time.Second {
		return 15 * time.Second
	}
	return d
}

// decodeB64 is a small helper used by file transfer.
func decodeB64(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }
