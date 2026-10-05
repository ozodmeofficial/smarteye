//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

// nativeWindow is false off Windows; the dashboard opens in a browser in dev.
const nativeWindow = false

// runNativeWindow is a no-op off Windows.
func runNativeWindow(url, title string) bool { return false }

// openDashboard opens the dashboard URL in the default browser on dev machines.
func openDashboard(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
