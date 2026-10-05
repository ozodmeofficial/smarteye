//go:build !windows

package osops

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"sync"
)

// Info returns best-effort system info on non-Windows dev machines.
func Info() SysInfo {
	host, _ := os.Hostname()
	name := ""
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	return SysInfo{Hostname: host, Username: name, OS: runtime.GOOS}
}

// ActiveWindow is unavailable off Windows.
func ActiveWindow() Foreground { return Foreground{App: "", Title: ""} }

// EnsureFirewall is a no-op off Windows.
func EnsureFirewall() {}

// Battery returns -1 (unknown) off Windows.
func Battery() int { return -1 }

// Power is a no-op stub off Windows.
func Power(action string, delaySec int) error {
	return fmt.Errorf("osops: power %q not supported on this platform", action)
}

// Launch is a no-op stub off Windows.
func Launch(target string, args []string, isURL bool) error {
	return fmt.Errorf("osops: launch not supported on this platform")
}

// ShowMessage prints to stderr off Windows.
func ShowMessage(title, body string, timeoutSec int) error {
	fmt.Fprintf(os.Stderr, "[SmartEYE message] %s: %s\n", title, body)
	return nil
}

// stubLocker tracks lock state in memory for dev builds.
type stubLocker struct {
	mu     sync.Mutex
	locked bool
}

func newLocker() Locker { return &stubLocker{} }

func (s *stubLocker) Show(title, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.locked = true
	fmt.Fprintf(os.Stderr, "[SmartEYE lock] %s — %s\n", title, message)
	return nil
}

func (s *stubLocker) Hide() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.locked = false
	return nil
}

func (s *stubLocker) IsLocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.locked
}
