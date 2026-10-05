// Package osops wraps the operating-system actions the SmartEYE client agent
// performs on command: reporting system info, powering off, launching apps,
// showing messages, and presenting the full-screen lock overlay.
//
// Only the Windows build does the real work; other platforms get safe stubs so
// the project builds and the agent can be exercised during development.
package osops

// SysInfo describes the client machine for the server's device list.
type SysInfo struct {
	Hostname string
	Username string
	OS       string
}

// Foreground describes the active window.
type Foreground struct {
	App   string // executable name, e.g. "chrome.exe"
	Title string // window title
}

// Locker owns the full-screen lock overlay. Show is idempotent; calling it while
// locked updates the displayed text. Hide removes the overlay.
type Locker interface {
	Show(title, message string) error
	Hide() error
	IsLocked() bool
}

// NewLocker returns the platform lock overlay controller.
func NewLocker() Locker { return newLocker() }
