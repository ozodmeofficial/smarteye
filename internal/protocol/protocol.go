// Package protocol defines the wire format shared by the SmartEYE server
// (o'qituvchi/rahbar paneli) and the SmartEYE client agent (o'quvchi/xodim).
//
// Every message is a JSON object with a "type" field and a "payload" field.
// The transport is a TLS WebSocket; this package is transport-agnostic and
// only concerns itself with (de)serialization and the message vocabulary.
package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// Version is bumped whenever the wire format changes incompatibly. The server
// refuses clients whose major version differs from its own.
const Version = "1.0.0"

// MessageType enumerates every message that can travel between peers.
type MessageType string

const (
	// --- Handshake / lifecycle ---------------------------------------------
	TypeHello        MessageType = "hello"         // client -> server, first frame after connect
	TypeWelcome      MessageType = "welcome"       // server -> client, accepted
	TypeReject       MessageType = "reject"        // server -> client, refused (bad code/version)
	TypePing         MessageType = "ping"          // either direction, keep-alive
	TypePong         MessageType = "pong"          // reply to ping
	TypeBye          MessageType = "bye"           // graceful disconnect notice
	TypeClientUpdate MessageType = "client_update" // client -> server, changed status (foreground app, etc.)

	// --- Live thumbnails & streaming ---------------------------------------
	TypeThumbRequest MessageType = "thumb_request" // server -> client, start/stop low-fps preview
	TypeThumbFrame   MessageType = "thumb_frame"   // client -> server, a preview frame
	TypeStreamStart  MessageType = "stream_start"  // server -> client, begin full-quality stream
	TypeStreamStop   MessageType = "stream_stop"   // server -> client, end full stream
	TypeStreamFrame  MessageType = "stream_frame"  // client -> server, a full-quality frame

	// --- Remote control (AnyDesk-style) ------------------------------------
	TypeControlMode  MessageType = "control_mode"  // server -> client, view-only vs control
	TypeInputEvent   MessageType = "input_event"   // server -> client, mouse/keyboard event
	TypeClipboard    MessageType = "clipboard"     // either direction, shared clipboard text
	TypeMonitorList  MessageType = "monitor_list"  // client -> server, available displays
	TypeSelectMon    MessageType = "select_monitor"// server -> client, choose a display

	// --- Supervision (Veyon-style) -----------------------------------------
	TypeLock      MessageType = "lock"       // server -> client, show lock screen
	TypeUnlock    MessageType = "unlock"     // server -> client, remove lock screen
	TypeMessage   MessageType = "message"    // server -> client, popup message
	TypePower     MessageType = "power"      // server -> client, shutdown/reboot/logoff
	TypeLaunch    MessageType = "launch"     // server -> client, open app or URL
	TypeDemoStart MessageType = "demo_start" // server -> client, show teacher's screen fullscreen
	TypeDemoStop  MessageType = "demo_stop"  // server -> client, end demo
	TypeDemoFrame MessageType = "demo_frame" // server -> client, a frame of the teacher screen

	// --- File transfer -----------------------------------------------------
	TypeFileOffer  MessageType = "file_offer"  // either direction, metadata of an incoming file
	TypeFileChunk  MessageType = "file_chunk"  // either direction, a base64 slice of file data
	TypeFileDone   MessageType = "file_done"   // either direction, transfer complete
	TypeFileAccept MessageType = "file_accept" // receiver -> sender, ok to send

	// --- Errors ------------------------------------------------------------
	TypeError MessageType = "error"
)

// Envelope is the outer frame for every message on the wire.
type Envelope struct {
	Type    MessageType     `json:"type"`
	ID      string          `json:"id,omitempty"`   // optional correlation id
	Payload json.RawMessage `json:"payload,omitempty"`
	Sent    int64           `json:"sent"` // unix millis, for latency measurement
}

// NewEnvelope marshals payload into an Envelope of the given type.
func NewEnvelope(t MessageType, payload any) (*Envelope, error) {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("protocol: marshal %s payload: %w", t, err)
		}
		raw = b
	}
	return &Envelope{Type: t, Payload: raw, Sent: time.Now().UnixMilli()}, nil
}

// Decode unmarshals the envelope payload into v.
func (e *Envelope) Decode(v any) error {
	if len(e.Payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(e.Payload, v); err != nil {
		return fmt.Errorf("protocol: decode %s payload: %w", e.Type, err)
	}
	return nil
}

// Latency returns how long ago the message was sent, from the receiver's clock.
func (e *Envelope) Latency() time.Duration {
	return time.Since(time.UnixMilli(e.Sent))
}
