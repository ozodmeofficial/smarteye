package server

import (
	"encoding/json"
	"log"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/ozodmeofficial/smarteye/internal/protocol"
)

// browserMsg is the envelope exchanged with the web UI over WebSocket.
type browserMsg struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// browserConn is one connected dashboard (a browser tab).
type browserConn struct {
	ws  *websocket.Conn
	hub *Hub
	out chan browserMsg
}

// send enqueues a message for the browser, dropping it if the buffer is full so
// a slow tab never blocks the hub.
func (b *browserConn) send(msg browserMsg) {
	select {
	case b.out <- msg:
	default:
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// --- Browser registry -------------------------------------------------------

func (h *Hub) addBrowser(b *browserConn) {
	h.mu.Lock()
	first := len(h.browsers) == 0
	h.browsers[b] = struct{}{}
	h.mu.Unlock()
	if first {
		h.setThumbnailsEnabled(true)
	}
	// Send the current state to the newcomer.
	b.send(browserMsg{Type: "snapshot", Payload: mustJSON(h.snapshot())})
}

func (h *Hub) removeBrowser(b *browserConn) {
	h.mu.Lock()
	delete(h.browsers, b)
	for _, vs := range h.viewers {
		delete(vs, b)
	}
	last := len(h.browsers) == 0
	h.mu.Unlock()
	if last {
		h.setThumbnailsEnabled(false)
	}
}

func (h *Hub) pushToAll(msg browserMsg) {
	h.mu.RLock()
	targets := make([]*browserConn, 0, len(h.browsers))
	for b := range h.browsers {
		targets = append(targets, b)
	}
	h.mu.RUnlock()
	for _, b := range targets {
		b.send(msg)
	}
}

// broadcastState pushes a fresh device/room snapshot to every browser.
func (h *Hub) broadcastState() {
	h.pushToAll(browserMsg{Type: "snapshot", Payload: mustJSON(h.snapshot())})
}

// --- Thumbnails -------------------------------------------------------------

// setThumbnailsEnabled turns the low-rate preview on/off across all online
// agents. Previews run only while at least one dashboard is open.
func (h *Hub) setThumbnailsEnabled(on bool) {
	req := protocol.ThumbRequest{Enabled: on, FPS: 1, MaxWidth: 360, Quality: 55}
	h.mu.RLock()
	conns := make([]*protocol.Conn, 0, len(h.devices))
	for _, d := range h.devices {
		if d.conn != nil {
			conns = append(conns, d.conn)
		}
	}
	h.mu.RUnlock()
	for _, c := range conns {
		_ = c.SendTyped(protocol.TypeThumbRequest, req)
	}
}

// enableThumbForConn turns previews on for one freshly connected agent, if any
// dashboard is open.
func (h *Hub) enableThumbForConn(c *protocol.Conn) {
	h.mu.RLock()
	open := len(h.browsers) > 0
	h.mu.RUnlock()
	if open {
		_ = c.SendTyped(protocol.TypeThumbRequest, protocol.ThumbRequest{
			Enabled: true, FPS: 1, MaxWidth: 360, Quality: 55,
		})
	}
}

// --- Command senders --------------------------------------------------------

func (h *Hub) deviceConn(id string) *protocol.Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if d := h.devices[id]; d != nil {
		return d.conn
	}
	return nil
}

func (h *Hub) sendTo(id string, t protocol.MessageType, payload any) {
	if c := h.deviceConn(id); c != nil {
		_ = c.SendTyped(t, payload)
	}
}

func (h *Hub) sendToMany(ids []string, t protocol.MessageType, payload any) {
	for _, id := range ids {
		h.sendTo(id, t, payload)
	}
}

// startView registers a browser as a full-quality viewer of a device and asks
// the agent to begin streaming.
func (h *Hub) startView(b *browserConn, id string, monitor, fps, quality int) {
	h.mu.Lock()
	if h.viewers[id] == nil {
		h.viewers[id] = map[*browserConn]struct{}{}
	}
	first := len(h.viewers[id]) == 0
	h.viewers[id][b] = struct{}{}
	h.mu.Unlock()
	if first {
		h.sendTo(id, protocol.TypeStreamStart, protocol.StreamStart{Monitor: monitor, FPS: fps, Quality: quality})
	}
}

// stopView removes a browser as a viewer and stops the agent stream if it was
// the last one.
func (h *Hub) stopView(b *browserConn, id string) {
	h.mu.Lock()
	if vs := h.viewers[id]; vs != nil {
		delete(vs, b)
		if len(vs) == 0 {
			delete(h.viewers, id)
			h.mu.Unlock()
			h.sendTo(id, protocol.TypeStreamStop, nil)
			return
		}
	}
	h.mu.Unlock()
}

// --- Browser message handling -----------------------------------------------

// handleBrowserMsg dispatches a command from the dashboard.
func (h *Hub) handleBrowserMsg(b *browserConn, msg browserMsg) {
	switch msg.Type {
	case "ping":
		b.send(browserMsg{Type: "pong"})

	// --- rooms ---
	case "create_room":
		var p struct {
			Name string `json:"name"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.mu.Lock()
		id := uuid.NewString()
		h.rooms[id] = &Room{ID: id, Name: p.Name}
		h.mu.Unlock()
		h.persist()
		h.broadcastState()

	case "rename_room":
		var p struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.mu.Lock()
		if r := h.rooms[p.ID]; r != nil {
			r.Name = p.Name
		}
		h.mu.Unlock()
		h.persist()
		h.broadcastState()

	case "delete_room":
		var p struct {
			ID string `json:"id"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.mu.Lock()
		delete(h.rooms, p.ID)
		for dev, room := range h.roomOf {
			if room == p.ID {
				delete(h.roomOf, dev)
				if d := h.devices[dev]; d != nil {
					d.Room = ""
				}
			}
		}
		h.mu.Unlock()
		h.persist()
		h.broadcastState()

	case "assign_room":
		var p struct {
			Devices []string `json:"devices"`
			Room    string   `json:"room"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.mu.Lock()
		for _, dev := range p.Devices {
			if p.Room == "" {
				delete(h.roomOf, dev)
			} else {
				h.roomOf[dev] = p.Room
			}
			if d := h.devices[dev]; d != nil {
				d.Room = p.Room
			}
		}
		h.mu.Unlock()
		h.persist()
		h.broadcastState()

	// --- supervision ---
	case "lock":
		var p struct {
			Targets []string `json:"targets"`
			Title   string   `json:"title"`
			Message string   `json:"message"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.sendToMany(p.Targets, protocol.TypeLock, protocol.Lock{Title: p.Title, Message: p.Message})

	case "unlock":
		var p struct {
			Targets []string `json:"targets"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.sendToMany(p.Targets, protocol.TypeUnlock, nil)

	case "message":
		var p struct {
			Targets []string `json:"targets"`
			Title   string   `json:"title"`
			Body    string   `json:"body"`
			Timeout int      `json:"timeout"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.sendToMany(p.Targets, protocol.TypeMessage, protocol.Message{Title: p.Title, Body: p.Body, Timeout: p.Timeout})

	case "power":
		var p struct {
			Targets []string `json:"targets"`
			Action  string   `json:"action"`
			Delay   int      `json:"delay"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.sendToMany(p.Targets, protocol.TypePower, protocol.Power{Action: p.Action, Delay: p.Delay})

	case "launch":
		var p struct {
			Targets []string `json:"targets"`
			Target  string   `json:"target"`
			IsURL   bool     `json:"is_url"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.sendToMany(p.Targets, protocol.TypeLaunch, protocol.Launch{Target: p.Target, IsURL: p.IsURL})

	// --- remote view & control ---
	case "view_start":
		var p struct {
			Device  string `json:"device"`
			Monitor int    `json:"monitor"`
			FPS     int    `json:"fps"`
			Quality int    `json:"quality"`
		}
		json.Unmarshal(msg.Payload, &p)
		if p.FPS == 0 {
			p.FPS = 20
		}
		if p.Quality == 0 {
			p.Quality = 70
		}
		h.startView(b, p.Device, p.Monitor, p.FPS, p.Quality)

	case "view_stop":
		var p struct {
			Device string `json:"device"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.stopView(b, p.Device)

	case "select_monitor":
		var p struct {
			Device  string `json:"device"`
			Monitor int    `json:"monitor"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.sendTo(p.Device, protocol.TypeSelectMon, protocol.SelectMonitor{Monitor: p.Monitor})

	case "control":
		var p struct {
			Device  string `json:"device"`
			Control bool   `json:"control"`
			Freeze  bool   `json:"freeze"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.sendTo(p.Device, protocol.TypeControlMode, protocol.ControlMode{
			Control: p.Control, FreezeClient: p.Freeze, ShowBadge: true,
		})

	case "input":
		var p struct {
			Device string              `json:"device"`
			Event  protocol.InputEvent `json:"event"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.sendTo(p.Device, protocol.TypeInputEvent, p.Event)

	case "clipboard":
		var p struct {
			Device string `json:"device"`
			Text   string `json:"text"`
		}
		json.Unmarshal(msg.Payload, &p)
		h.sendTo(p.Device, protocol.TypeClipboard, protocol.Clipboard{Text: p.Text})

	case "probe_ip":
		// Manual "search by IP": send unicast discovery beacons to the given
		// addresses/ranges so clients on a broadcast-isolated LAN still connect.
		var p struct {
			IPs []string `json:"ips"`
		}
		json.Unmarshal(msg.Payload, &p)
		if h.probe != nil && len(p.IPs) > 0 {
			go h.probe(p.IPs)
		}

	default:
		log.Printf("server: unknown browser command %q", msg.Type)
	}
}
