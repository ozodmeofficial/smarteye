// Package config loads and persists SmartEYE settings and the stable device
// identity. The role (server or client) is chosen once, at install time, and
// stored here so the same binary behaves differently per machine.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Role is what a given installation acts as.
type Role string

const (
	RoleServer Role = "server" // o'qituvchi / rahbar paneli
	RoleClient Role = "client" // o'quvchi / xodim agenti
)

// Config is the on-disk configuration, shared by both roles.
type Config struct {
	Role       Role   `json:"role"`
	DeviceID   string `json:"device_id"`   // stable unique id for this machine
	ServerName string `json:"server_name"` // display name (server role)
	NetCode    string `json:"net_code"`    // pairing code shared across the fleet

	// Client-only hints.
	ServerHost string `json:"server_host"` // optional manual server address (fallback)
	ShowBadge  bool   `json:"show_badge"`  // show the "under supervision" indicator

	// UI preferences (server role).
	Language string `json:"language"` // "uz","ru","en"
	Theme    string `json:"theme"`    // "light","dark","system"

	// Networking.
	ListenPort    int `json:"listen_port"`    // TLS WebSocket port (server)
	DiscoveryPort int `json:"discovery_port"` // UDP discovery port

	path string     `json:"-"`
	mu   sync.Mutex `json:"-"`
}

// Defaults for a fresh install.
const (
	DefaultListenPort    = 47800
	DefaultDiscoveryPort = 47801
)

// Dir returns the per-user config directory for SmartEYE, creating it.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		base, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	dir := filepath.Join(base, "SmartEYE")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("config: create dir: %w", err)
	}
	return dir, nil
}

// Path returns the path of the config file.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config file, creating a default one (with a fresh device id)
// if none exists yet.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	c := &Config{path: p}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		c.applyDefaults()
		return c, c.Save()
	}
	if err != nil {
		return nil, fmt.Errorf("config: read: %w", err)
	}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("config: parse: %w", err)
	}
	c.path = p
	c.applyDefaults() // fill any missing fields added by newer versions
	return c, nil
}

func (c *Config) applyDefaults() {
	if c.DeviceID == "" {
		c.DeviceID = newDeviceID()
	}
	if c.Language == "" {
		c.Language = "uz"
	}
	if c.Theme == "" {
		c.Theme = "system"
	}
	if c.ListenPort == 0 {
		c.ListenPort = DefaultListenPort
	}
	if c.DiscoveryPort == 0 {
		c.DiscoveryPort = DefaultDiscoveryPort
	}
	if c.Role == RoleClient && !c.badgeExplicit() {
		// Supervision must be visible by default; openness is a core principle.
		c.ShowBadge = true
	}
}

// badgeExplicit reports whether ShowBadge was already set true. It exists so a
// later Save never silently flips an admin's explicit choice; the default only
// applies on first creation.
func (c *Config) badgeExplicit() bool { return c.ShowBadge }

// Save persists the config atomically.
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path == "" {
		p, err := Path()
		if err != nil {
			return err
		}
		c.path = p
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("config: write tmp: %w", err)
	}
	return os.Rename(tmp, c.path)
}

// IsConfigured reports whether a role has been chosen.
func (c *Config) IsConfigured() bool {
	return c.Role == RoleServer || c.Role == RoleClient
}

// newDeviceID returns a random 128-bit hex identifier.
func newDeviceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// rand should never fail; fall back to a timestamp-ish value.
		return "dev-" + hex.EncodeToString([]byte(fmt.Sprintf("%d", os.Getpid())))
	}
	return hex.EncodeToString(b)
}

// NormalizeNetCode strips spaces and lowercases a pairing code so "482 913"
// and "482913" compare equal.
func NormalizeNetCode(code string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(code), " ", ""))
}
