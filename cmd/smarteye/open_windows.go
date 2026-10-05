//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// openDashboard opens the local dashboard. It prefers Edge/Chrome in "app"
// mode so the control panel looks like a dedicated window rather than a browser
// tab, falling back to the default browser.
func openDashboard(url string) {
	appArg := "--app=" + url
	candidates := [][]string{
		{"msedge", appArg},
		{"chrome", appArg},
	}
	for _, c := range candidates {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false}
		if err := cmd.Start(); err == nil {
			return
		}
	}
	// Fallback: default browser via the shell.
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}
