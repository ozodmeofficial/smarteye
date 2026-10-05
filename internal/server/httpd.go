package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ozodmeofficial/smarteye/internal/meta"
)

//go:embed all:web
var webFS embed.FS

// router wires the dashboard's static files, info endpoint and WebSocket.
func (s *Server) router() http.Handler {
	mux := http.NewServeMux()

	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(sub)))

	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"server_name": s.cfg.ServerName,
			"net_code":    s.cfg.NetCode,
			"version":     meta.Version,
			"language":    s.cfg.Language,
			"theme":       s.cfg.Theme,
			"listen_port": s.cfg.ListenPort,
		})
	})

	mux.HandleFunc("/api/prefs", func(w http.ResponseWriter, r *http.Request) {
		// Persist UI preferences (language/theme) chosen in the dashboard.
		var p struct {
			Language string `json:"language"`
			Theme    string `json:"theme"`
		}
		if json.NewDecoder(r.Body).Decode(&p) == nil {
			if p.Language != "" {
				s.cfg.Language = p.Language
			}
			if p.Theme != "" {
				s.cfg.Theme = p.Theme
			}
			_ = s.cfg.Save()
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/ws", s.handleWS)
	return mux
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 1 << 20,
	// The dashboard is served from loopback only; accept same-origin upgrades.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// handleWS upgrades a dashboard connection and runs its read/write pumps.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	b := &browserConn{ws: ws, hub: s.hub, out: make(chan browserMsg, 256)}
	s.hub.addBrowser(b)
	defer func() {
		s.hub.removeBrowser(b)
		ws.Close()
	}()

	// Writer pump.
	go func() {
		for msg := range b.out {
			ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := ws.WriteJSON(msg); err != nil {
				ws.Close()
				return
			}
		}
	}()

	// Reader pump.
	ws.SetReadLimit(4 << 20)
	for {
		var msg browserMsg
		if err := ws.ReadJSON(&msg); err != nil {
			close(b.out)
			return
		}
		s.hub.handleBrowserMsg(b, msg)
	}
}
