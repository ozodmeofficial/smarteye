//go:build windows

package main

import (
	"os/exec"
	"runtime"
	"syscall"

	webview "github.com/jchv/go-webview2"
)

// nativeWindow reports that this platform can host the dashboard in its own
// application window instead of a web browser.
const nativeWindow = true

// runNativeWindow opens the dashboard inside an embedded WebView2 window — a
// real SmartEYE application window, not a browser tab. It blocks until the
// window is closed. Returns false if WebView2 is unavailable, so the caller can
// fall back to a browser.
func runNativeWindow(url, title string) bool {
	runtime.LockOSThread()
	w := webview.NewWithOptions(webview.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview.WindowOptions{
			Title:  title,
			Width:  1280,
			Height: 820,
			Center: true,
		},
	})
	if w == nil {
		return false // WebView2 runtime not installed
	}
	defer w.Destroy()
	w.Navigate(url)
	w.Run()
	return true
}

// openDashboard is the browser fallback used only when the native window cannot
// be created.
func openDashboard(url string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}
