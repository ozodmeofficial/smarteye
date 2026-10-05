//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

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
