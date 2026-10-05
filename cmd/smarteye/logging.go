package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/ozodmeofficial/smarteye/internal/config"
)

// setupLogging writes logs to a rotating-ish file in the config directory and,
// when a console is attached, also to stderr. On Windows the app is linked as a
// GUI binary (no console), so the file is the primary record.
func setupLogging() {
	dir, err := config.Dir()
	if err != nil {
		return
	}
	logPath := filepath.Join(dir, "smarteye.log")
	// Truncate if the log grew beyond ~2 MB to avoid unbounded growth.
	if info, err := os.Stat(logPath); err == nil && info.Size() > 2<<20 {
		_ = os.Remove(logPath)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	// Combine file + stderr when stderr is usable.
	var w io.Writer = f
	if isConsole(os.Stderr) {
		w = io.MultiWriter(f, os.Stderr)
	}
	log.SetOutput(w)
}

// isConsole reports whether the given file looks like an interactive console.
func isConsole(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
