// Package control injects mouse and keyboard input on the client so the server
// operator can drive the remote machine (AnyDesk-style). The real backend is
// Windows SendInput; elsewhere a no-op backend keeps the build portable.
package control

import "github.com/ozodmeofficial/smarteye/internal/protocol"

// Injector replays input events onto the local machine.
type Injector interface {
	// Apply performs a single input event. Coordinates in the event are
	// normalized 0..1 against the given monitor width/height.
	Apply(ev protocol.InputEvent, monW, monH, monX, monY int) error
	// SetClipboard sets the system clipboard text.
	SetClipboard(text string) error
	// GetClipboard reads the system clipboard text.
	GetClipboard() (string, error)
}

// New returns the platform injector.
func New() Injector { return newInjector() }
