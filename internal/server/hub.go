// Package server implements the SmartEYE server: the teacher's / manager's
// side. It accepts agent connections over TLS, keeps a live model of every
// device and room, serves the web UI, and routes commands and frames between
// the browser dashboard and the agents.
package server

import (
	"sync"
	"time"

	"github.com/ozodmeofficial/smarteye/internal/protocol"
)

// Device is the server's live view of one connected client.
type Device struct {
	ID         string `json:"id"`
	Hostname   string `json:"hostname"`
	Username   string `json:"username"`
	OS         string `json:"os"`
	AppVersion string `json:"app_version"`
	Addr       string `json:"addr"`
	Room       string `json:"room"`

	Online          bool      `json:"online"`
	LastSeen        time.Time `json:"last_seen"`
	ForegroundApp   string    `json:"foreground_app"`
	ForegroundTitle string    `json:"foreground_title"`
	Locked          bool      `json:"locked"`
	Battery         int       `json:"battery"`
	Monitors        []protocol.MonitorInfo `json:"monitors"`

	// Latency to the device, measured from pings, in milliseconds.
	LatencyMS int64 `json:"latency_ms"`

	// conn is the live agent connection; nil when offline. Not serialized.
	conn *protocol.Conn `json:"-"`

	// latest preview frame (base64 JPEG data URL body), guarded by Hub.mu.
	thumb    string `json:"-"`
	thumbSeq uint64 `json:"-"`
}

// Room groups devices (e.g. "1-xona", "Buxgalteriya").
type Room struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Hub is the central registry of devices, rooms and browser subscribers.
type Hub struct {
	mu       sync.RWMutex
	devices  map[string]*Device // by device ID
	rooms    map[string]*Room   // by room ID
	roomOf   map[string]string  // deviceID -> roomID (persisted)
	browsers map[*browserConn]struct{}

	// viewers maps a deviceID to the set of browsers currently streaming it at
	// full quality, so the hub knows when to start/stop the agent stream.
	viewers map[string]map[*browserConn]struct{}

	store *roomStore
}

// NewHub creates an empty hub backed by the given room store.
func NewHub(store *roomStore) *Hub {
	h := &Hub{
		devices:  map[string]*Device{},
		rooms:    map[string]*Room{},
		roomOf:   map[string]string{},
		browsers: map[*browserConn]struct{}{},
		viewers:  map[string]map[*browserConn]struct{}{},
		store:    store,
	}
	if store != nil {
		rooms, assign := store.Load()
		for _, r := range rooms {
			rr := r
			h.rooms[r.ID] = &rr
		}
		for dev, room := range assign {
			h.roomOf[dev] = room
		}
	}
	return h
}

// --- Device lifecycle -------------------------------------------------------

// register adds or revives a device on agent connect. Returns the device and
// the room it was filed into.
func (h *Hub) register(hello protocol.Hello, addr string, conn *protocol.Conn) *Device {
	h.mu.Lock()
	defer h.mu.Unlock()

	d := h.devices[hello.DeviceID]
	if d == nil {
		d = &Device{ID: hello.DeviceID, Battery: -1}
		h.devices[hello.DeviceID] = d
	}
	d.Hostname = hello.Hostname
	d.Username = hello.Username
	d.OS = hello.OS
	d.AppVersion = hello.AppVersion
	d.Addr = addr
	d.Online = true
	d.LastSeen = time.Now()
	d.conn = conn
	d.Room = h.roomOf[hello.DeviceID]
	return d
}

// unregister marks a device offline when its agent disconnects.
func (h *Hub) unregister(id string, conn *protocol.Conn) {
	h.mu.Lock()
	d := h.devices[id]
	if d != nil && d.conn == conn {
		d.Online = false
		d.conn = nil
		d.LastSeen = time.Now()
	}
	delete(h.viewers, id)
	h.mu.Unlock()
	h.broadcastState()
}

// updateStatus applies a ClientUpdate from an agent.
func (h *Hub) updateStatus(id string, u protocol.ClientUpdate) {
	h.mu.Lock()
	if d := h.devices[id]; d != nil {
		d.ForegroundApp = u.ForegroundApp
		d.ForegroundTitle = u.ForegroundTitle
		d.Locked = u.Locked
		d.Battery = u.BatteryPercent
		d.LastSeen = time.Now()
	}
	h.mu.Unlock()
	h.broadcastState()
}

// setMonitors records the display list reported by an agent.
func (h *Hub) setMonitors(id string, mons []protocol.MonitorInfo) {
	h.mu.Lock()
	if d := h.devices[id]; d != nil {
		d.Monitors = mons
	}
	h.mu.Unlock()
	h.broadcastState()
}

// setThumb stores the latest preview frame and forwards it to subscribers.
func (h *Hub) setThumb(id string, frame protocol.Frame) {
	h.mu.Lock()
	d := h.devices[id]
	if d == nil {
		h.mu.Unlock()
		return
	}
	d.thumb = frame.Data
	d.thumbSeq = frame.Seq
	d.LastSeen = time.Now()
	h.mu.Unlock()

	h.pushToAll(browserMsg{
		Type: "thumb",
		Payload: mustJSON(map[string]any{
			"id":     id,
			"data":   frame.Data,
			"width":  frame.Width,
			"height": frame.Height,
			"seq":    frame.Seq,
		}),
	})
}

// forwardStreamFrame routes a full-quality frame to the browsers viewing it.
func (h *Hub) forwardStreamFrame(id string, frame protocol.Frame) {
	h.mu.RLock()
	viewers := h.viewers[id]
	targets := make([]*browserConn, 0, len(viewers))
	for b := range viewers {
		targets = append(targets, b)
	}
	h.mu.RUnlock()

	msg := browserMsg{
		Type: "frame",
		Payload: mustJSON(map[string]any{
			"id":      id,
			"data":    frame.Data,
			"width":   frame.Width,
			"height":  frame.Height,
			"monitor": frame.Monitor,
			"seq":     frame.Seq,
		}),
	}
	for _, b := range targets {
		b.send(msg)
	}
}

// setLatency records a ping round-trip for a device.
func (h *Hub) setLatency(id string, ms int64) {
	h.mu.Lock()
	if d := h.devices[id]; d != nil {
		d.LatencyMS = ms
	}
	h.mu.Unlock()
}

// --- Snapshots --------------------------------------------------------------

// snapshot is the full state pushed to browsers.
type snapshot struct {
	Devices []*Device `json:"devices"`
	Rooms   []*Room   `json:"rooms"`
}

func (h *Hub) snapshot() snapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	devs := make([]*Device, 0, len(h.devices))
	for _, d := range h.devices {
		// Copy without the heavy thumb field and without the conn.
		dc := *d
		dc.conn = nil
		dc.thumb = ""
		devs = append(devs, &dc)
	}
	rooms := make([]*Room, 0, len(h.rooms))
	for _, r := range h.rooms {
		rooms = append(rooms, r)
	}
	return snapshot{Devices: devs, Rooms: rooms}
}
