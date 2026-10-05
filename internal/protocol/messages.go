package protocol

// This file defines the concrete payload structs carried inside an Envelope.

// --- Handshake --------------------------------------------------------------

// Hello is the first frame a client sends after the TLS WebSocket opens.
type Hello struct {
	Version    string `json:"version"`     // protocol version of the client
	DeviceID   string `json:"device_id"`   // stable unique id (survives IP changes)
	Hostname   string `json:"hostname"`    // computer name
	Username   string `json:"username"`    // logged-in OS user
	OS         string `json:"os"`          // e.g. "Windows 10 Pro 22H2"
	AppVersion string `json:"app_version"` // SmartEYE build version
	NetCode    string `json:"net_code"`    // hashed pairing code proving same network/owner
}

// Welcome is sent by the server when a client is accepted.
type Welcome struct {
	ServerName string `json:"server_name"`
	DeviceID   string `json:"device_id"` // echoed back, confirmed
	// Room is the room the server has already filed this device into (may be empty).
	Room string `json:"room"`
}

// Reject explains why a client was refused.
type Reject struct {
	Reason string `json:"reason"` // "bad_code", "version_mismatch", "banned"
	Detail string `json:"detail"`
}

// ClientUpdate reports a change in the client's live status.
type ClientUpdate struct {
	ForegroundApp   string `json:"foreground_app"`   // active window app name
	ForegroundTitle string `json:"foreground_title"` // active window title
	Locked          bool   `json:"locked"`           // whether the lock screen is showing
	BatteryPercent  int    `json:"battery_percent"`  // -1 if no battery
	Idle            bool   `json:"idle"`             // no input for a while
}

// --- Thumbnails & streaming -------------------------------------------------

// ThumbRequest toggles the low-frequency preview used for the grid tiles.
type ThumbRequest struct {
	Enabled  bool `json:"enabled"`
	FPS      int  `json:"fps"`       // frames per second (1-2 typical)
	MaxWidth int  `json:"max_width"` // downscale target, e.g. 320
	Quality  int  `json:"quality"`   // JPEG quality 1-100
}

// Frame carries an encoded image. Used by thumb, stream and demo frames.
type Frame struct {
	Monitor int    `json:"monitor"` // which display this frame is from
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Format  string `json:"format"` // "jpeg" or "webp"
	Full    bool   `json:"full"`   // true = full frame, false = dirty-rect delta
	X       int    `json:"x"`      // delta position (when Full is false)
	Y       int    `json:"y"`
	Data    string `json:"data"` // base64-encoded image bytes
	Seq     uint64 `json:"seq"`  // monotonically increasing per stream
}

// StreamStart asks the client to begin a full-quality stream of a monitor.
type StreamStart struct {
	Monitor int `json:"monitor"`
	FPS     int `json:"fps"`     // target frames per second (e.g. 25)
	Quality int `json:"quality"` // JPEG quality 1-100
}

// --- Remote control ---------------------------------------------------------

// ControlMode switches a remote session between view-only and full control.
type ControlMode struct {
	Control      bool `json:"control"`       // true = server drives input
	FreezeClient bool `json:"freeze_client"` // true = disable local mouse/keyboard
	ShowBadge    bool `json:"show_badge"`    // show "teacher connected" indicator
}

// InputEvent is a single mouse or keyboard action to replay on the client.
type InputEvent struct {
	Kind    string  `json:"kind"` // "mouse_move","mouse_down","mouse_up","wheel","key_down","key_up"
	X       float64 `json:"x"`    // normalized 0..1 coordinates (resolution independent)
	Y       float64 `json:"y"`
	Button  string  `json:"button"`   // "left","right","middle"
	Delta   int     `json:"delta"`    // wheel delta
	KeyCode int     `json:"key_code"` // virtual key code
	Key     string  `json:"key"`      // human key name, for logging
	Mods    uint8   `json:"mods"`     // bitmask: 1=ctrl 2=alt 4=shift 8=win
}

// Clipboard carries shared clipboard text between peers.
type Clipboard struct {
	Text string `json:"text"`
}

// MonitorInfo describes one physical display on the client.
type MonitorInfo struct {
	Index   int  `json:"index"`
	Width   int  `json:"width"`
	Height  int  `json:"height"`
	Primary bool `json:"primary"`
}

// MonitorList is the set of displays a client has.
type MonitorList struct {
	Monitors []MonitorInfo `json:"monitors"`
}

// SelectMonitor tells the client which display to capture.
type SelectMonitor struct {
	Monitor int `json:"monitor"`
}

// --- Supervision ------------------------------------------------------------

// Lock shows a full-screen lock overlay with an optional message.
type Lock struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

// Message shows a popup/toast on the client.
type Message struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Timeout int    `json:"timeout"` // seconds; 0 = until dismissed
}

// Power requests a power action.
type Power struct {
	Action string `json:"action"` // "shutdown","reboot","logoff","sleep"
	Delay  int    `json:"delay"`  // seconds before executing
}

// Launch opens an application or URL on the client.
type Launch struct {
	Target string   `json:"target"` // path/exe or URL
	Args   []string `json:"args"`
	IsURL  bool     `json:"is_url"`
}

// DemoStart puts the client into demo mode (teacher screen fullscreen).
type DemoStart struct {
	AllowInput bool `json:"allow_input"` // whether students can still use their own input
}

// --- File transfer ----------------------------------------------------------

// FileOffer announces a file about to be sent.
type FileOffer struct {
	TransferID string `json:"transfer_id"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Mime       string `json:"mime"`
}

// FileAccept approves an offered transfer.
type FileAccept struct {
	TransferID string `json:"transfer_id"`
	Accept     bool   `json:"accept"`
}

// FileChunk carries one slice of file bytes.
type FileChunk struct {
	TransferID string `json:"transfer_id"`
	Seq        int    `json:"seq"`
	Data       string `json:"data"` // base64
}

// FileDone marks the end of a transfer.
type FileDone struct {
	TransferID string `json:"transfer_id"`
	Checksum   string `json:"checksum"` // sha256 hex
}

// --- Errors -----------------------------------------------------------------

// Error is a generic error payload.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Modifier bit constants for InputEvent.Mods.
const (
	ModCtrl  uint8 = 1 << 0
	ModAlt   uint8 = 1 << 1
	ModShift uint8 = 1 << 2
	ModWin   uint8 = 1 << 3
)
